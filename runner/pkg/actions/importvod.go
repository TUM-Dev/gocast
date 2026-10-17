package actions

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"os/exec"
	"path"
	"strconv"
	"time"

	"github.com/tum-dev/gocast/runner/config"
	"github.com/tum-dev/gocast/runner/pkg/ffmpeg"
	"github.com/tum-dev/gocast/runner/pkg/metrics"
	"github.com/tum-dev/gocast/runner/pkg/ptr"
	"github.com/tum-dev/gocast/runner/protobuf"
)

// maxCopyLevel is the highest h264 level (4.2) that browsers decode reliably. Uploads
// above it are transcoded even if the codec is right.
const maxCopyLevel = 42

// ImportVOD turns an uploaded video file into a VOD: it is repackaged into HLS without
// re-encoding when browsers can play it as is, and transcoded otherwise. The result
// lands where MkVOD puts the VODs of recorded streams, so the edge serves both alike.
//
// The uploaded file is removed afterwards, whether or not the import worked. Every
// failure is aborting: retrying a file ffmpeg rejected once only burns CPU.
func ImportVOD(ctx context.Context, logger *slog.Logger, notify chan *protobuf.Notification, d map[string]any, metrics *metrics.Broker) error {
	streamID, ok := d["streamID"].(uint64)
	if !ok {
		return AbortingError(fmt.Errorf("no stream id in context"))
	}
	streamVersion, ok := d["streamVersion"].(string)
	if !ok {
		return AbortingError(fmt.Errorf("no stream version in context"))
	}
	uploadPath, ok := d["uploadPath"].(string)
	if !ok || uploadPath == "" {
		return AbortingError(fmt.Errorf("no upload path in context"))
	}
	defer func() {
		if err := os.Remove(uploadPath); err != nil && !os.IsNotExist(err) {
			logger.Warn("couldn't remove uploaded file", "file", uploadPath, "err", err)
		}
	}()

	metrics.ConvertingProgresses.With(metrics.With().Stream(streamID).L()).Inc()
	defer metrics.ConvertingProgresses.With(metrics.With().Stream(streamID).L()).Dec()

	probe, err := ffmpeg.Probe(ctx, uploadPath)
	if err != nil {
		return AbortingError(fmt.Errorf("probe upload: %w", err))
	}
	if probe.Codec("video") == "" {
		return AbortingError(fmt.Errorf("upload has no video stream"))
	}
	transcode := needsTranscode(probe.Codec("video"), probe.Codec("audio"), probe.Level())

	vodDir := path.Join(config.Config.StoragePath, fmt.Sprintf("%d", streamID), streamVersion)
	// Written next to the final directory and swapped in at the end: a failed re-upload
	// must leave the previous VOD intact, and a half-written one must never be served.
	workDir := vodDir + ".importing"
	if err := os.RemoveAll(workDir); err != nil {
		return AbortingError(fmt.Errorf("clear work directory: %w", err))
	}
	if err := os.MkdirAll(workDir, os.ModePerm); err != nil {
		return AbortingError(fmt.Errorf("create work directory: %w", err))
	}
	defer func() { _ = os.RemoveAll(workDir) }() // no-op once renamed

	logger.Info("importing upload", "transcode", transcode, "file", uploadPath)
	// An argument slice, not a split string: the file name comes from the uploader.
	command := exec.CommandContext(ctx, "ffmpeg", importArgs(uploadPath, workDir, transcode)...)
	command.WaitDelay = 10 * time.Second
	stderr, err := command.StderrPipe()
	if err != nil {
		return AbortingError(err)
	}
	go logCmdPipe(logger, stderr, []any{"stream", streamID, "logStream", "stderr"})
	if err := command.Run(); err != nil {
		return AbortingError(fmt.Errorf("ffmpeg failed: %w", err))
	}

	// A re-upload replaces the previous VOD.
	if err := os.RemoveAll(vodDir); err != nil {
		return AbortingError(fmt.Errorf("remove previous VOD: %w", err))
	}
	if err := os.Rename(workDir, vodDir); err != nil {
		return AbortingError(fmt.Errorf("publish VOD: %w", err))
	}

	vodPath := path.Join(vodDir, "playlist.m3u8")
	// The thumbnail action reads the VOD from here.
	d["playlistURL"] = vodPath

	vodURL, err := url.JoinPath(config.Config.EdgeServer, "vod", fmt.Sprintf("%d", streamID), streamVersion, "playlist.m3u8")
	if err != nil {
		return AbortingError(fmt.Errorf("join vod url: %w", err))
	}
	notify <- &protobuf.Notification{
		Data: &protobuf.Notification_VodReady{
			VodReady: &protobuf.VODReadyNotification{
				Stream:        &protobuf.StreamInfo{Id: ptr.Take(streamID)},
				StreamVersion: ptr.Take(protobuf.StreamVersion(protobuf.StreamVersion_value[streamVersion])),
				Url:           ptr.Take(vodURL),
			},
		},
	}
	return nil
}

// needsTranscode reports whether an upload can't be repackaged as is. The container
// doesn't matter, only the codecs do: HLS segments are rewritten either way. A missing
// audio track is fine, a missing level is not, as the video can't be shown to be playable.
func needsTranscode(videoCodec, audioCodec, level string) bool {
	if videoCodec != "h264" {
		return true
	}
	if audioCodec != "" && audioCodec != "aac" {
		return true
	}
	l, err := strconv.Atoi(level)
	return err != nil || l > maxCopyLevel
}

// importArgs builds the ffmpeg arguments that write input as a VOD playlist into vodDir.
func importArgs(input, vodDir string, transcode bool) []string {
	args := []string{"-nostdin", "-y", "-i", input}
	if transcode {
		args = append(args,
			// Even dimensions, as yuv420p requires them.
			"-vf", "scale=trunc(iw/2)*2:trunc(ih/2)*2",
			"-c:v", "libx264", "-preset", "veryfast", "-crf", "23", "-pix_fmt", "yuv420p",
			// Keyframes on the segment boundaries keep the segments close to hls_time.
			"-force_key_frames", "expr:gte(t,n_forced*20)",
			"-c:a", "aac",
		)
	} else {
		args = append(args, "-c", "copy")
	}
	return append(args,
		"-f", "hls",
		"-hls_time", "20",
		"-hls_playlist_type", "vod",
		"-hls_segment_filename", path.Join(vodDir, "%05d.ts"),
		path.Join(vodDir, "playlist.m3u8"),
	)
}
