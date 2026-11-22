package actions

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path"

	"github.com/tum-dev/gocast/runner/pkg/metrics"
	"github.com/tum-dev/gocast/runner/protobuf"
)

// VoDPendingFile marks a recording whose VoD is due but not yet being written. A runner
// that starts makes the VoD of every recording it finds the file in, which is how a VoD
// survives a restart while the storage is down. Its content is a PendingVoD.
//
// MkVOD removes it right before it writes to the storage: its ffmpeg appends to an
// existing playlist, so a VoD redone after a crash halfway through would hold the
// beginning twice. Those recordings are left to a human, as before.
const VoDPendingFile = ".vod-pending.json"

// PendingVoD is what the VoD actions of a recording need besides its directory.
type PendingVoD struct {
	StreamID      uint64 `json:"streamID"`
	StreamVersion string `json:"streamVersion"`
	DetectSilence bool   `json:"detectSilence"`
}

// MarkVoDPending writes the VoDPendingFile of a recording. It is the first VoD action, as
// a discarded recording skips them: no VoD may be made of it after a restart either.
// Its errors are aborting, so it is not retried, but the VoD actions after it still run,
// see Action: failing it only costs the resumption.
func MarkVoDPending(_ context.Context, _ *slog.Logger, _ chan *protobuf.Notification, d map[string]any, _ *metrics.Broker) error {
	recordingDir, ok := d["recordingDir"].(string)
	if !ok {
		return AbortingError(fmt.Errorf("no recordingDir in context"))
	}
	streamID, ok := d["streamID"].(uint64)
	if !ok {
		return AbortingError(fmt.Errorf("no stream id in context"))
	}
	streamVersion, ok := d["streamVersion"].(string)
	if !ok {
		return AbortingError(fmt.Errorf("no stream version in context"))
	}
	detectSilence, _ := d["detectSilence"].(bool)
	b, err := json.Marshal(PendingVoD{StreamID: streamID, StreamVersion: streamVersion, DetectSilence: detectSilence})
	if err != nil {
		return AbortingError(err)
	}
	if err := os.WriteFile(path.Join(recordingDir, VoDPendingFile), b, 0o644); err != nil {
		return AbortingError(fmt.Errorf("write %s: %w", VoDPendingFile, err))
	}
	return nil
}

// ReadPendingVoD reads a VoDPendingFile.
func ReadPendingVoD(file string) (PendingVoD, error) {
	var p PendingVoD
	b, err := os.ReadFile(file)
	if err != nil {
		return p, err
	}
	if err := json.Unmarshal(b, &p); err != nil {
		return p, fmt.Errorf("parse %s: %w", file, err)
	}
	if p.StreamID == 0 {
		return p, fmt.Errorf("%s has no stream id", file)
	}
	if _, ok := protobuf.StreamVersion_value[p.StreamVersion]; !ok {
		return p, fmt.Errorf("%s has unknown stream version %q", file, p.StreamVersion)
	}
	return p, nil
}

// unmarkPendingVoD removes the VoDPendingFile of a recording, if there is one.
func unmarkPendingVoD(recordingDir string) error {
	if err := os.Remove(path.Join(recordingDir, VoDPendingFile)); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// vodPending tells whether a recording still has its VoDPendingFile.
func vodPending(recordingDir string) bool {
	_, err := os.Stat(path.Join(recordingDir, VoDPendingFile))
	return err == nil
}
