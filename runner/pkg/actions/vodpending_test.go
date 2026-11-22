package actions

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tum-dev/gocast/runner/config"
)

// pendingRecording is a recording directory with a VoDPendingFile, as MarkVoDPending
// leaves it, and the job data of its VoD actions.
func pendingRecording(t *testing.T) (string, map[string]any) {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "1", "STREAM_VERSION_COMBINED")
	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		t.Fatal(err)
	}
	d := map[string]any{"streamID": uint64(1), "streamVersion": "STREAM_VERSION_COMBINED", "recordingDir": dir}
	if err := MarkVoDPending(context.Background(), nil, nil, d, nil); err != nil {
		t.Fatal(err)
	}
	return dir, d
}

func managementFiles(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".del-") || strings.HasPrefix(e.Name(), ".keep-") {
			names = append(names, e.Name())
		}
	}
	return names
}

var quiet = slog.New(slog.NewTextHandler(io.Discard, nil))

// MkVOD gave up before writing, so the next start makes the VoD: a .keep would have the
// recording backed up and removed under it.
func TestCheckVoDLeavesAPendingVoDAlone(t *testing.T) {
	dir, d := pendingRecording(t)
	if err := CheckVoD(context.Background(), quiet, nil, d, testBroker()); err == nil {
		t.Error("CheckVoD succeeded without a VoD")
	}
	if got := managementFiles(t, dir); len(got) > 0 {
		t.Errorf("CheckVoD wrote %v next to the VoDPendingFile", got)
	}
}

// Once MkVOD started writing, nothing resumes the VoD, so a failed one must get its .keep
// even when the job was cancelled, as on a forced shutdown during the conversion.
func TestCheckVoDKeepsAnUnfinishedVoD(t *testing.T) {
	dir, d := pendingRecording(t)
	if err := unmarkPendingVoD(dir); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_ = CheckVoD(ctx, quiet, nil, d, testBroker())
	if got := managementFiles(t, dir); len(got) != 1 || !strings.HasPrefix(got[0], ".keep-") {
		t.Errorf("management files = %v, want one .keep", got)
	}
}

func withStoragePath(t *testing.T, p string) {
	t.Helper()
	prev := config.Config.StoragePath
	config.Config.StoragePath = p
	t.Cleanup(func() { config.Config.StoragePath = prev })
}

// A storage error that is not an outage is a human's: the recording is unmarked, so
// CheckVoD's .keep reaches them and the next start does not make the VoD under the backup.
func TestMkVODUnmarksOnPermanentStorageErrors(t *testing.T) {
	dir, d := pendingRecording(t)
	notADir := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(notADir, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	withStoragePath(t, notADir)

	if err := MkVOD(context.Background(), quiet, nil, d, testBroker()); !IsAbortingError(err) {
		t.Fatalf("err = %v, want an aborting error", err)
	}
	if vodPending(dir) {
		t.Error("the recording is still marked pending")
	}
}

// Giving up on drain leaves the VoD to the next start, which needs the marker.
func TestMkVODKeepsTheMarkerOnDrain(t *testing.T) {
	prevTimeout, prevGate := storageTimeout, storage
	storageTimeout = 10 * time.Millisecond
	t.Cleanup(func() { storageTimeout, storage = prevTimeout, prevGate })
	storage = &storageGate{inFlight: make(chan struct{})} // the storage is down
	withStoragePath(t, t.TempDir())

	dir, d := pendingRecording(t)
	drained := make(chan struct{})
	close(drained)
	d["drained"] = (<-chan struct{})(drained)

	if err := MkVOD(context.Background(), quiet, nil, d, testBroker()); !IsAbortingError(err) {
		t.Fatalf("err = %v, want an aborting error", err)
	}
	if !vodPending(dir) {
		t.Error("the marker is gone, the next start won't make the VoD")
	}
}
