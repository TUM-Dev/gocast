package actions

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/tum-dev/gocast/runner/pkg/metrics"
	"github.com/tum-dev/gocast/runner/protobuf"
)

// DiscardGrace is how long a discarded recording is kept before livestreamCleanup removes
// it. Once the VoD actions are skipped the recording is the only copy left, so a discard
// that was clicked by mistake stays recoverable by hand for this long.
const DiscardGrace = 2 * time.Hour

// DiscardRecording marks the live recording for deletion instead of turning it into a VoD.
// It runs in place of MkVOD/CheckVoD/MkThumb when a stream was ended with discardVod.
//
// Without it the recording directory carries neither a .del- nor a .keep- marker and is not
// empty either, so livestreamCleanup never looks at it and it stays on SEGMENT_PATH forever.
func DiscardRecording(_ context.Context, log *slog.Logger, _ chan *protobuf.Notification, d map[string]any, _ *metrics.Broker) error {
	recordingDir, ok := d["recordingDir"].(string)
	if !ok {
		return AbortingError(fmt.Errorf("no recordingDir in context"))
	}
	deleteAt := time.Now().Add(DiscardGrace)
	log.Info("marking discarded recording for deletion", "dir", recordingDir, "at", deleteAt)
	return writeManagementFile(recordingDir, fmt.Sprintf(".del-%d", deleteAt.Unix()))
}
