package runner

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/tum-dev/gocast/runner/config"
	"github.com/tum-dev/gocast/runner/pkg/actions"
)

// withSegmentPath points config.Config.SegmentPath at a fresh temp dir for one test.
func withSegmentPath(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	old := config.Config.SegmentPath
	config.Config.SegmentPath = dir
	t.Cleanup(func() { config.Config.SegmentPath = old })
	return dir
}

func writeRecording(t *testing.T, segmentPath string, name string) string {
	t.Helper()
	dir := filepath.Join(segmentPath, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "playlist.m3u8"), []byte("#EXTM3U\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// A recording that was never marked is invisible to the cleanup: it has no .del-/.keep-
// marker and is not empty either. That is how a discarded recording used to stay on
// SEGMENT_PATH forever.
func TestDetermineCleanableDirectoriesIgnoresUnmarkedRecordings(t *testing.T) {
	segmentPath := withSegmentPath(t)
	dir := writeRecording(t, segmentPath, "42/COMB")

	remove, backup, err := determineCleanableDirectories(segmentPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := remove[dir]; ok {
		t.Errorf("unmarked recording %s is scheduled for removal", dir)
	}
	if _, ok := backup[dir]; ok {
		t.Errorf("unmarked recording %s is scheduled for backup", dir)
	}
}

// DiscardRecording's marker is what hands the recording to livestreamCleanup, after the
// grace period has passed.
func TestDiscardedRecordingIsCleanedUpAfterTheGracePeriod(t *testing.T) {
	segmentPath := withSegmentPath(t)
	dir := writeRecording(t, segmentPath, "42/COMB")

	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	if err := actions.DiscardRecording(context.Background(), log, nil, map[string]any{"recordingDir": dir}, nil); err != nil {
		t.Fatalf("DiscardRecording: %v", err)
	}

	remove, backup, err := determineCleanableDirectories(segmentPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := backup[dir]; ok {
		t.Errorf("discarded recording %s was scheduled for backup, discarding is not an error", dir)
	}
	deleteAt, ok := remove[dir]
	if !ok {
		t.Fatalf("discarded recording %s is not scheduled for removal, got %v", dir, remove)
	}
	// livestreamCleanup skips entries whose time has not come yet, so the recording
	// survives the grace period and is removed afterwards.
	if !deleteAt.After(time.Now()) {
		t.Errorf("discarded recording is removed immediately (%s), want it kept for %s", deleteAt, actions.DiscardGrace)
	}
	if deleteAt.After(time.Now().Add(actions.DiscardGrace + time.Minute)) {
		t.Errorf("discarded recording is kept until %s, longer than the %s grace period", deleteAt, actions.DiscardGrace)
	}
}
