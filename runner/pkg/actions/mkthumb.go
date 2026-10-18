package actions

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"strconv"
	"strings"

	"github.com/tum-dev/gocast/runner/pkg/metrics"
	"github.com/tum-dev/gocast/runner/pkg/ptr"
	"github.com/tum-dev/gocast/runner/protobuf"
)

// MkThumb Generates a thumbnail from a stream and sends it to the control plane in a notification
func MkThumb(_ context.Context, logger *slog.Logger, notify chan *protobuf.Notification, d map[string]any, metrics *metrics.Broker) error {
	logger.Info("generating thumbnail")
	streamID, ok := d["streamID"].(uint64)
	if !ok {
		return AbortingError(fmt.Errorf("no stream id in context"))
	}
	var streamVersion *protobuf.StreamVersion
	if sv, ok := d["streamVersion"].(string); ok {
		streamVersion = ptr.Take(protobuf.StreamVersion(protobuf.StreamVersion_value[sv]))
	} else {
		return AbortingError(fmt.Errorf("no stream version in context"))
	}
	recordingDir, ok := d["recordingDir"].(string)
	if !ok {
		return AbortingError(fmt.Errorf("no recordingDir in context"))
	}

	thumb, err := createVideoThumbnail(recordingDir + "/playlist.m3u8")
	if err != nil {
		return AbortingError(fmt.Errorf("error creating thumbnail: %v", err))
	}

	notify <- &protobuf.Notification{
		Data: &protobuf.Notification_ThumbnailReady{ThumbnailReady: &protobuf.ThumbnailReadyNotification{
			Stream:        &protobuf.StreamInfo{Id: ptr.Take(streamID)},
			StreamVersion: streamVersion,
			Thumbnail:     thumb,
		}},
	}
	return nil
}

// thumbnailWidth is the width of the generated thumbnail in pixels.
const thumbnailWidth = 720

// createVideoThumbnail creates a thumbnail from the middle of the given video or playlist.
//
// The seek deliberately comes after the input. A self-stream whose publisher reconnected is one
// playlist of several pieces, each with timestamps restarting near zero (marked with
// #EXT-X-DISCONTINUITY). An input seek, which thumbgen uses, goes by those timestamps: half
// the playlist's duration can lie beyond the end of every piece, and ffmpeg then finds no
// frame at all. Seeking on the output decodes from the start and counts decoded time, which
// is what the playlist's duration is made of.
func createVideoThumbnail(source string) ([]byte, error) {
	duration, err := probeDuration(source)
	if err != nil {
		return nil, err
	}
	thumb, err := frameAt(source, duration/2)
	if err != nil || len(thumb) == 0 {
		// The duration is only as good as the playlist's own bookkeeping, so rather
		// no thumbnail from the middle than none at all.
		thumb, err = frameAt(source, 0)
	}
	if err != nil {
		return nil, err
	}
	if len(thumb) == 0 {
		return nil, fmt.Errorf("no frame in %s", source)
	}
	return thumb, nil
}

func probeDuration(source string) (float64, error) {
	out, err := exec.Command("ffprobe", "-v", "error", "-show_entries", "format=duration", "-of", "csv=p=0", source).Output()
	if err != nil {
		return 0, fmt.Errorf("ffprobe: %w", err)
	}
	d, err := strconv.ParseFloat(strings.TrimSpace(string(out)), 64)
	if err != nil {
		return 0, fmt.Errorf("duration %q of %s: %w", out, source, err)
	}
	return d, nil
}

// frameAt returns the first frame at or after the given decoded time as a JPEG, scaled to
// thumbnailWidth. It is empty if the input ends before that.
func frameAt(source string, seconds float64) ([]byte, error) {
	cmd := exec.Command("ffmpeg", "-v", "error", "-i", source, "-ss", strconv.FormatFloat(seconds, 'f', 3, 64),
		"-vf", fmt.Sprintf("scale=%d:-1", thumbnailWidth), "-frames:v", "1", "-f", "image2pipe", "-c:v", "mjpeg", "pipe:1")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("ffmpeg: %s: %w", stderr.String(), err)
	}
	return out, nil
}
