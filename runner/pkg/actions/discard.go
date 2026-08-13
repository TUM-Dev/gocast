package actions

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/tum-dev/gocast/runner/pkg/metrics"
	"github.com/tum-dev/gocast/runner/protobuf"
)

// DiscardRecording marks the live recording for deletion instead of turning it into a VoD.
// It runs in place of MkVOD/CheckVoD/MkThumb when a stream was ended with discardVod.
func DiscardRecording(_ context.Context, log *slog.Logger, _ chan *protobuf.Notification, d map[string]any, _ *metrics.Broker) error {
	recordingDir, ok := d["recordingDir"].(string)
	if !ok {
		return AbortingError(fmt.Errorf("no recordingDir in context"))
	}
	log.Info("marking recording for deletion", "dir", recordingDir)
	return writeManagementFile(recordingDir, fmt.Sprintf(".del-%d", time.Now().Unix()))
}
