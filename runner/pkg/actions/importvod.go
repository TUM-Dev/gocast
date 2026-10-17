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
	"strings"
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
	transcode := needsTranscode(probe.Codec("video"), probe.Codec("audio"), probe.Level(), probe.PixelFormat())

	vodDir := path.Join(config.Config.StoragePath, fmt.Sprintf("%d", streamID), streamVersion)
	// Written next to the final directory and swapped in at the end: a failed re-upload
	// must leave the previous VOD intact, and a half-written one must never be served.
	// Unique per import, so two uploads of the same stream version can't wipe each
	// other's output.
	if err := os.MkdirAll(path.Dir(vodDir), os.ModePerm); err != nil {
		return AbortingError(fmt.Errorf("create stream directory: %w", err))
	}
	workDir, err := os.MkdirTemp(path.Dir(vodDir), ".importing-")
	if err != nil {
		return AbortingError(fmt.Errorf("create work directory: %w", err))
	}
	defer func() { _ = os.RemoveAll(workDir) }() // no-op once renamed

	logger.Info("importing upload", "transcode", transcode, "file", uploadPath)
	// An argument slice, not a split string: the file name comes from the uploader.
	command := exec.CommandContext(ctx, "ffmpeg", importArgs(uploadPath, workDir, transcode)...)
	command.WaitDelay = 10 * time.Second
	// Not StderrPipe: Wait closes the pipe, so reading it concurrently can lose the end
	// of the output, which is where ffmpeg says why it failed.
	var stderr tailBuffer
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		return AbortingError(fmt.Errorf("ffmpeg failed: %w: %s", err, stderr.String()))
	}

	if err := replaceVOD(workDir, vodDir); err != nil {
		return AbortingError(err)
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
// The pixel format matters too: 10 bit and 4:2:2/4:4:4 H.264, which cameras and screen
// recorders produce, have the right codec name and the right level, yet browsers can't
// decode them.
func needsTranscode(videoCodec, audioCodec, level, pixFmt string) bool {
	if videoCodec != "h264" {
		return true
	}
	if pixFmt != "yuv420p" && pixFmt != "yuvj420p" {
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

// replaceVOD puts the finished import at vodDir, replacing a previous VOD. The old one is
// moved aside rather than deleted first, so a failure leaves it in place, and viewers
// only miss the playlist for the time between two renames.
func replaceVOD(workDir, vodDir string) error {
	var trash string
	if _, err := os.Stat(vodDir); err == nil {
		trash, err = os.MkdirTemp(path.Dir(vodDir), ".replaced-")
		if err != nil {
			return fmt.Errorf("create trash directory: %w", err)
		}
		defer func() { _ = os.RemoveAll(trash) }()
		if err := os.Rename(vodDir, path.Join(trash, "vod")); err != nil {
			return fmt.Errorf("move previous VOD aside: %w", err)
		}
	}
	if err := os.Rename(workDir, vodDir); err != nil {
		if trash != "" {
			_ = os.Rename(path.Join(trash, "vod"), vodDir) // best effort: keep the old VOD
		}
		return fmt.Errorf("publish VOD: %w", err)
	}
	return nil
}

// tailBuffer keeps the last bytes written to it: the end of ffmpeg's output is the part
// that holds the error.
type tailBuffer struct{ b []byte }

const tailBufferSize = 4096

func (t *tailBuffer) Write(p []byte) (int, error) {
	t.b = append(t.b, p...)
	if len(t.b) > tailBufferSize {
		t.b = t.b[len(t.b)-tailBufferSize:]
	}
	return len(p), nil
}

func (t *tailBuffer) String() string { return strings.TrimSpace(string(t.b)) }
