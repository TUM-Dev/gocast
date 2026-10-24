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
func MkThumb(ctx context.Context, logger *slog.Logger, notify chan *protobuf.Notification, d map[string]any, metrics *metrics.Broker) error {
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

	thumb, err := createVideoThumbnail(ctx, logger, recordingDir+"/playlist.m3u8")
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
// It first seeks on the input, which for HLS fetches only the segment that holds the middle:
// about a second, where seeking on the output downloads and decodes everything before it
// (minutes for a 100 minute lecture, read over HTTP). Redoing a course asked for hundreds of
// those at once.
//
// The input seek goes by the timestamps in the segments, and a self-stream whose publisher
// reconnected is one playlist of several pieces, each with timestamps restarting near zero
// (marked with #EXT-X-DISCONTINUITY). Half the playlist's duration can lie beyond the end of
// every piece, and ffmpeg then finds no frame at all and fails. For those the fallback seeks
// on the output, which decodes from the start and counts decoded time, which is what the
// playlist's duration is made of.
//
// The middle is only where a representative frame is likely, not a requirement. Where an
// input seek on such a playlist does not fail but lands inside one piece, that frame is as
// good as any, so it is not told apart from the real middle.
//
// The fallback is logged: it costs minutes of CPU per thumbnail, and if the input seek ever
// starts failing for another reason, the old load would come back unnoticed.
func createVideoThumbnail(ctx context.Context, logger *slog.Logger, source string) ([]byte, error) {
	duration, err := probeDuration(ctx, source)
	if err != nil {
		return nil, err
	}
	thumb, err := frameAt(ctx, source, duration/2, true)
	if err != nil || len(thumb) == 0 {
		// A cancelled job fails every attempt at once, and its error should say so.
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		logger.Info("no frame from the input seek, seeking on the output", "source", source, "err", err)
		thumb, err = frameAt(ctx, source, duration/2, false)
	}
	if err != nil || len(thumb) == 0 {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		// The duration is only as good as the playlist's own bookkeeping, so rather
		// no thumbnail from the middle than none at all.
		thumb, err = frameAt(ctx, source, 0, false)
	}
	if err != nil {
		return nil, err
	}
	if len(thumb) == 0 {
		return nil, fmt.Errorf("no frame in %s", source)
	}
	return thumb, nil
}

func probeDuration(ctx context.Context, source string) (float64, error) {
	out, err := exec.CommandContext(ctx, "ffprobe", "-v", "error", "-show_entries", "format=duration", "-of", "csv=p=0", source).Output()
	if err != nil {
		return 0, fmt.Errorf("ffprobe: %w", err)
	}
	d, err := strconv.ParseFloat(strings.TrimSpace(string(out)), 64)
	if err != nil {
		return 0, fmt.Errorf("duration %q of %s: %w", out, source, err)
	}
	return d, nil
}

// frameAt returns the first frame at or after the given time as a JPEG, scaled to
// thumbnailWidth. It is empty if the input ends before that. seekInput chooses where ffmpeg
// seeks, see createVideoThumbnail.
func frameAt(ctx context.Context, source string, seconds float64, seekInput bool) ([]byte, error) {
	ss := []string{"-ss", strconv.FormatFloat(seconds, 'f', 3, 64)}
	args := []string{"-v", "error"}
	if seekInput {
		args = append(args, ss...)
	}
	args = append(args, "-i", source)
	if !seekInput {
		args = append(args, ss...)
	}
	args = append(args, "-vf", fmt.Sprintf("scale=%d:-1", thumbnailWidth), "-frames:v", "1", "-f", "image2pipe", "-c:v", "mjpeg", "pipe:1")
	cmd := exec.CommandContext(ctx, "ffmpeg", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("ffmpeg: %s: %w", stderr.String(), err)
	}
	return out, nil
}
