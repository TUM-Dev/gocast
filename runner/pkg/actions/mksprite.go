package actions

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"math"
	"os/exec"
	"strconv"
	"time"

	"github.com/tum-dev/gocast/runner/pkg/metrics"
	"github.com/tum-dev/gocast/runner/pkg/ptr"
	"github.com/tum-dev/gocast/runner/protobuf"
)

const (
	// spriteIntervalSeconds is the time between two frames. It is fixed because a stream
	// stores one interval for the sprites of all its versions, and versions of slightly
	// different length must not end up on different sides of a threshold.
	spriteIntervalSeconds = 30
	// The frame layout the player assumes, see frontend/src/lib/player/sprites.ts.
	spriteFrameWidth  = 160
	spriteFrameHeight = 90
	spriteColumns     = 17
	// spriteTimeout bounds one sheet, see silenceScanTimeout.
	spriteTimeout = time.Hour
	// spriteMaxBytes keeps a sheet within gRPC's default 4 MiB limit on the manager's side,
	// which a stream of several hours can exceed. A notification over it would never be
	// accepted, and the runner would keep retrying it.
	spriteMaxBytes = 3_500_000
)

// MkSprite makes the sheet of seekbar preview frames and reports it in a
// SpriteReadyNotification.
//
// A failure is logged and otherwise ignored. Previews are a convenience for viewers, so it
// must not fail a job whose VoD is made and already published, and a redo of the
// post-processing can fix it.
func MkSprite(ctx context.Context, logger *slog.Logger, notify chan *protobuf.Notification, d map[string]any, _ *metrics.Broker) error {
	streamID, ok := d["streamID"].(uint64)
	if !ok {
		return AbortingError(fmt.Errorf("no stream id in context"))
	}
	sv, ok := d["streamVersion"].(string)
	if !ok {
		return AbortingError(fmt.Errorf("no stream version in context"))
	}
	input, err := vodInput(d)
	if err != nil {
		return AbortingError(err)
	}

	// Waited for before the sheet's own timeout starts, which is only for the sheet.
	release, err := acquirePostProcessing(ctx)
	if err != nil {
		logger.Error("sprite skipped, the job ended while it waited", "err", err)
		return nil
	}
	defer release()

	logger.Info("generating sprite", "input", input)
	sprite, err := createSprite(ctx, input)
	if err != nil {
		logger.Error("sprite failed", "err", err)
		return nil
	}
	if len(sprite) > spriteMaxBytes {
		logger.Error("sprite too large to report", "bytes", len(sprite))
		return nil
	}
	notify <- &protobuf.Notification{Data: &protobuf.Notification_SpriteReady{SpriteReady: &protobuf.SpriteReadyNotification{
		Stream:          &protobuf.StreamInfo{Id: ptr.Take(streamID)},
		StreamVersion:   ptr.Take(protobuf.StreamVersion(protobuf.StreamVersion_value[sv])),
		Sprite:          sprite,
		IntervalSeconds: ptr.Take(uint32(spriteIntervalSeconds)),
	}}}
	return nil
}

func createSprite(ctx context.Context, input string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, spriteTimeout)
	defer cancel()
	duration, err := probeDuration(ctx, input)
	if err != nil {
		return nil, err
	}
	if duration <= 0 {
		return nil, fmt.Errorf("duration of %f seconds", duration)
	}

	// -skip_frame nokey decodes the keyframes only. The fps filter then picks the last keyframe
	// of each interval it rounds to, which lands near the middle of the 30 seconds a cell
	// stands for in the player.
	// -reinit_filter 0: a publisher who reconnects at another resolution would otherwise flush
	// tile, and -frames:v 1 would end the sheet there.
	args := []string{"-nostdin", "-v", "error", "-skip_frame", "nokey", "-reinit_filter", "0", "-i", input, "-an", "-sn",
		"-vf", spriteFilter(duration), "-frames:v", "1", "-q:v", "5", "-f", "image2pipe", "-c:v", "mjpeg", "pipe:1"}
	name := "ffmpeg"
	// Niced for the reason detectSilences is.
	if nice, err := exec.LookPath("nice"); err == nil {
		name, args = nice, append([]string{"ffmpeg"}, args...)
	}
	cmd := exec.CommandContext(ctx, name, args...)
	var out, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("ffmpeg: %w: %s", err, tail(stderr.String(), 500))
	}
	if out.Len() == 0 {
		return nil, fmt.Errorf("no frame in %s", input)
	}
	return out.Bytes(), nil
}

// spriteFilter takes a frame every interval, fits it into the frame size whatever the video's
// aspect ratio is, and lays the frames out in rows. Rows allow one frame past the duration: it
// comes from the playlist, and an overflowing frame would start a second sheet that is thrown
// away. eof_action=pass keeps the last frame, which fps otherwise drops when the video ends
// early in its interval, leaving the end of the seekbar with a black preview.
func spriteFilter(duration float64) string {
	frames := int(math.Ceil(duration/spriteIntervalSeconds)) + 1
	rows := (frames + spriteColumns - 1) / spriteColumns
	return "fps=1/" + strconv.Itoa(spriteIntervalSeconds) + ":eof_action=pass" +
		fmt.Sprintf(",scale=%d:%d:force_original_aspect_ratio=decrease", spriteFrameWidth, spriteFrameHeight) +
		fmt.Sprintf(",pad=%d:%d:(ow-iw)/2:(oh-ih)/2", spriteFrameWidth, spriteFrameHeight) +
		fmt.Sprintf(",tile=%dx%d", spriteColumns, rows)
}
