package runner

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/tum-dev/gocast/runner/config"
)

// A stream's directory is empty until ffmpeg writes the first segment. The cleanup must not
// remove it in that window: the stream fails with "Failed to open file .../00000.ts".
func TestEmptyDirectoryGrace(t *testing.T) {
	root := t.TempDir()
	prev := config.Config.SegmentPath
	config.Config.SegmentPath = root
	t.Cleanup(func() { config.Config.SegmentPath = prev })

	fresh := filepath.Join(root, "1", "STREAM_VERSION_COMBINED")
	stale := filepath.Join(root, "2", "STREAM_VERSION_CAMERA")
	for _, d := range []string{fresh, stale} {
		if err := os.MkdirAll(d, os.ModePerm); err != nil {
			t.Fatal(err)
		}
	}
	old := time.Now().Add(-emptyDirGrace - time.Minute)
	if err := os.Chtimes(stale, old, old); err != nil {
		t.Fatal(err)
	}

	remove, _, err := determineCleanableDirectories(root)
	if err != nil {
		t.Fatal(err)
	}

	now := time.Now()
	if due, ok := remove[fresh]; !ok || due.Before(now) {
		t.Errorf("a new stream's empty directory is due for removal now (due %v, found %v)", due, ok)
	}
	if due, ok := remove[stale]; !ok || !due.Before(now) {
		t.Errorf("a directory empty for longer than the grace is not due (due %v, found %v)", due, ok)
	}
}

// Once ffmpeg has written a segment, the directory is no longer a candidate at all.
func TestDirectoryWithSegmentsIsKept(t *testing.T) {
	root := t.TempDir()
	prev := config.Config.SegmentPath
	config.Config.SegmentPath = root
	t.Cleanup(func() { config.Config.SegmentPath = prev })

	dir := filepath.Join(root, "1", "STREAM_VERSION_PRESENTATION")
	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "00000.ts"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-2 * emptyDirGrace)
	if err := os.Chtimes(dir, old, old); err != nil {
		t.Fatal(err)
	}

	remove, _, err := determineCleanableDirectories(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := remove[dir]; ok {
		t.Error("a directory with segments is marked for removal")
	}
}
