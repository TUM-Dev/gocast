package actions

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/tum-dev/gocast/runner/config"
	"github.com/tum-dev/gocast/runner/pkg/metrics"
	"github.com/tum-dev/gocast/runner/pkg/ptr"
	"github.com/tum-dev/gocast/runner/protobuf"
)

// OfflinePackageRoot is where offline packages live, below the storage path the edge
// serves as /vod. It is a tree of its own rather than a directory in each VOD, because
// a re-upload replaces the VOD's directory as a whole (replaceVOD).
const OfflinePackageRoot = "offline"

// OfflinePackageDir is the directory of one offline package.
func OfflinePackageDir(packageID uint64) string {
	return path.Join(config.Config.StoragePath, OfflinePackageRoot, strconv.FormatUint(packageID, 10))
}

// Reasons an offline package failed, for the viewer who asked for it.
const (
	reasonOfflineNoVOD  = "The recording isn't available for offline viewing."
	reasonOfflineFailed = "The lecture couldn't be prepared for offline viewing. Please try again later."
)

// MkOfflinePackage copies a VOD into an encrypted HLS package a device downloads for
// offline viewing. The segments are copied, not re-encoded, and encrypted with HLS
// AES-128, which the device decrypts one segment at a time while playing. Every package
// has its own key, so one leaked key opens one person's copy.
func MkOfflinePackage(ctx context.Context, logger *slog.Logger, notify chan *protobuf.Notification, d map[string]any, _ *metrics.Broker) error {
	packageID, ok := d["packageID"].(uint64)
	if !ok || packageID == 0 {
		return AbortingError(fmt.Errorf("no package id in context"))
	}
	err := mkOfflinePackage(ctx, logger, notify, packageID, d)
	if err == nil {
		return nil
	}
	reason := reasonOfflineFailed
	var missing *offlineSourceMissingError
	if errors.As(err, &missing) {
		reason = reasonOfflineNoVOD
	}
	notify <- &protobuf.Notification{
		Data: &protobuf.Notification_OfflinePackageFailed{OfflinePackageFailed: &protobuf.OfflinePackageFailedNotification{
			PackageId: ptr.Take(packageID),
			Reason:    ptr.Take(reason),
		}},
	}
	// Reported already; a retry would only report it again.
	return AbortingError(err)
}

type offlineSourceMissingError struct{ err error }

func (e *offlineSourceMissingError) Error() string { return e.err.Error() }
func (e *offlineSourceMissingError) Unwrap() error { return e.err }

func mkOfflinePackage(ctx context.Context, logger *slog.Logger, notify chan *protobuf.Notification, packageID uint64, d map[string]any) error {
	streamID, ok := d["streamID"].(uint64)
	if !ok {
		return fmt.Errorf("no stream id in context")
	}
	streamVersion, ok := d["streamVersion"].(string)
	if !ok {
		return fmt.Errorf("no stream version in context")
	}
	key, ok := d["key"].([]byte)
	if !ok || len(key) != 16 {
		return fmt.Errorf("no 16 byte key in context")
	}
	keyURI, _ := d["keyURI"].(string)

	source := path.Join(config.Config.StoragePath, strconv.FormatUint(streamID, 10), streamVersion, "playlist.m3u8")
	if _, err := os.Stat(source); err != nil {
		return &offlineSourceMissingError{fmt.Errorf("source playlist: %w", err)}
	}

	pkgDir := OfflinePackageDir(packageID)
	if err := os.MkdirAll(path.Dir(pkgDir), os.ModePerm); err != nil {
		return fmt.Errorf("create offline directory: %w", err)
	}
	workDir, err := os.MkdirTemp(path.Dir(pkgDir), ".packaging-")
	if err != nil {
		return fmt.Errorf("create work directory: %w", err)
	}
	defer func() { _ = os.RemoveAll(workDir) }()

	// ffmpeg takes the key from a file named in a key info file. Both sit outside the
	// package, in a private temp directory, and go once ffmpeg is done.
	keyDir, err := os.MkdirTemp("", "offline-key-")
	if err != nil {
		return fmt.Errorf("create key directory: %w", err)
	}
	defer func() { _ = os.RemoveAll(keyDir) }()
	keyInfo, err := writeKeyInfo(keyDir, key, keyURI)
	if err != nil {
		return err
	}

	logger.Info("making offline package", "package", packageID, "stream", streamID, "version", streamVersion)
	command := exec.CommandContext(ctx, "ffmpeg", offlinePackageArgs(source, workDir, keyInfo)...)
	var stderr tailBuffer
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		return fmt.Errorf("ffmpeg failed: %w: %s", err, stderr.String())
	}

	segments, size, err := packageStats(workDir)
	if err != nil {
		return err
	}
	_ = os.RemoveAll(pkgDir) // a leftover of an earlier attempt for the same package
	if err := os.Rename(workDir, pkgDir); err != nil {
		return fmt.Errorf("publish offline package: %w", err)
	}

	notify <- &protobuf.Notification{
		Data: &protobuf.Notification_OfflinePackageReady{OfflinePackageReady: &protobuf.OfflinePackageReadyNotification{
			PackageId:    ptr.Take(packageID),
			SegmentCount: ptr.Take(segments),
			SizeBytes:    ptr.Take(size),
		}},
	}
	return nil
}

// writeKeyInfo writes the key and ffmpeg's key info file into dir and returns the
// latter's path. The info file has no IV line: ffmpeg then uses each segment's media
// sequence number, which is what HLS players assume, so the playlist needs none either.
func writeKeyInfo(dir string, key []byte, keyURI string) (string, error) {
	if keyURI == "" || strings.ContainsAny(keyURI, "\r\n") {
		keyURI = "offline-key"
	}
	keyFile := filepath.Join(dir, "key")
	if err := os.WriteFile(keyFile, key, 0o600); err != nil {
		return "", fmt.Errorf("write key: %w", err)
	}
	info := filepath.Join(dir, "keyinfo")
	if err := os.WriteFile(info, []byte(keyURI+"\n"+keyFile+"\n"), 0o600); err != nil {
		return "", fmt.Errorf("write key info: %w", err)
	}
	return info, nil
}

// offlinePackageArgs builds the ffmpeg arguments that copy source into an encrypted HLS
// playlist in dir. Segments are shorter than the VOD's 20 s so an interrupted download
// loses less, and the segment count stays small enough to fetch one by one.
func offlinePackageArgs(source, dir, keyInfo string) []string {
	return []string{
		"-nostdin", "-y",
		"-i", source,
		"-c", "copy",
		"-f", "hls",
		"-hls_time", "10",
		"-hls_playlist_type", "vod",
		"-hls_key_info_file", keyInfo,
		"-hls_segment_filename", path.Join(dir, "%05d.ts"),
		path.Join(dir, "playlist.m3u8"),
	}
}

// packageStats counts the segments in dir and sums the size of everything in it.
func packageStats(dir string) (segments uint32, size uint64, err error) {
	err = filepath.WalkDir(dir, func(p string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		size += uint64(info.Size())
		if strings.HasSuffix(p, ".ts") {
			segments++
		}
		return nil
	})
	if err != nil {
		return 0, 0, fmt.Errorf("measure offline package: %w", err)
	}
	if segments == 0 {
		return 0, 0, fmt.Errorf("offline package has no segments")
	}
	return segments, size, nil
}
