package actions

import (
	"context"
	"io"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
)

func discardLog() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestDiscardRecordingWritesDelMarkerAfterTheGracePeriod(t *testing.T) {
	dir := t.TempDir()
	before := time.Now()

	err := DiscardRecording(context.Background(), discardLog(), nil, map[string]any{"recordingDir": dir}, nil)
	if err != nil {
		t.Fatalf("DiscardRecording: %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("want exactly one marker file, got %v", entries)
	}
	name := entries[0].Name()
	if !strings.HasPrefix(name, ".del-") {
		t.Fatalf("marker %q is not the .del- marker livestreamCleanup looks for", name)
	}

	unixSec, err := strconv.ParseInt(strings.TrimPrefix(name, ".del-"), 10, 64)
	if err != nil {
		t.Fatalf("marker %q does not carry a unix timestamp: %v", name, err)
	}
	deleteAt := time.Unix(unixSec, 0)

	// the grace period is what makes a misclicked discard recoverable, deleting right away
	// would throw away the only copy that is left once VoD creation is skipped.
	if !deleteAt.After(time.Now()) {
		t.Errorf("recording is marked for immediate deletion (%s), want a grace period", deleteAt)
	}
	if want := before.Add(DiscardGrace); deleteAt.Before(want.Add(-time.Minute)) || deleteAt.After(want.Add(time.Minute)) {
		t.Errorf("deletion time %s is not roughly %s after the discard", deleteAt, DiscardGrace)
	}
}

func TestDiscardRecordingWithoutRecordingDirAborts(t *testing.T) {
	err := DiscardRecording(context.Background(), discardLog(), nil, map[string]any{}, nil)
	if err == nil {
		t.Fatal("want an error when recordingDir is missing")
	}
	// retrying cannot help, recordingDir is never added to d later on.
	if !IsAbortingError(err) {
		t.Errorf("error %v is not an AbortingError, RunAction would retry it forever", err)
	}
}
