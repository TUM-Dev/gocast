package runner

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRemoveStaleEntriesKeepsRecentPackages(t *testing.T) {
	root := t.TempDir()
	now := time.Now()
	for name, age := range map[string]time.Duration{"1": 25 * time.Hour, "2": time.Hour, ".packaging-x": 30 * time.Hour} {
		dir := filepath.Join(root, name)
		if err := os.Mkdir(dir, os.ModePerm); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(dir, now.Add(-age), now.Add(-age)); err != nil {
			t.Fatal(err)
		}
	}

	removeStaleEntries(root, 24*time.Hour, now, slog.Default())

	for name, want := range map[string]bool{"1": false, "2": true, ".packaging-x": false} {
		_, err := os.Stat(filepath.Join(root, name))
		if got := err == nil; got != want {
			t.Errorf("%s exists = %v, want %v", name, got, want)
		}
	}
	// A storage path without any offline package yet is not an error worth more than a log.
	removeStaleEntries(filepath.Join(root, "missing"), time.Hour, now, slog.Default())
}
