package actions

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/tum-dev/gocast/runner/pkg/metrics"
	"github.com/tum-dev/gocast/runner/pkg/ptr"
	"github.com/tum-dev/gocast/runner/protobuf"
)

// MkDuration probes the length of the finished VoD and reports it in a
// DurationProbedNotification. The server needs it to place viewers' watch positions and to
// show the length of a lecture, and nothing else knows it: the lecture's scheduled end says
// when the stream was meant to stop, not how long the recording is.
//
// A failed probe is logged and otherwise ignored. A missing length costs the lecture its
// duration in the lists, which a redo of the post-processing can fix, but it must not fail
// a job whose VoD is made and already published.
func MkDuration(ctx context.Context, logger *slog.Logger, notify chan *protobuf.Notification, d map[string]any, _ *metrics.Broker) error {
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

	// Redoing a course starts hundreds of these, and each reads the playlist over HTTP.
	release, err := acquirePostProcessing(ctx)
	if err != nil {
		logger.Error("duration probe skipped, the job ended while it waited", "err", err)
		return nil
	}
	defer release()

	duration, err := probeDuration(ctx, input)
	if err != nil {
		logger.Error("duration probe failed", "err", err)
		return nil
	}
	seconds := toSeconds(duration)
	if seconds == 0 {
		// The server takes 0 for "unknown", so reporting it would only replace a good
		// value with a worse one.
		logger.Warn("probed no duration", "input", input)
		return nil
	}
	logger.Info("probed duration", "seconds", seconds)
	notify <- &protobuf.Notification{
		Data: &protobuf.Notification_DurationProbed{DurationProbed: &protobuf.DurationProbedNotification{
			Stream:          &protobuf.StreamInfo{Id: ptr.Take(streamID)},
			StreamVersion:   ptr.Take(protobuf.StreamVersion(protobuf.StreamVersion_value[sv])),
			DurationSeconds: ptr.Take(seconds),
		}},
	}
	return nil
}
