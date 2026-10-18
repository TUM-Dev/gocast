package actions

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tum-dev/gocast/runner/config"
	"github.com/tum-dev/gocast/runner/protobuf"
)

// fakeFFmpeg puts an ffmpeg on PATH that records its arguments and exits with the given code.
func fakeFFmpeg(t *testing.T, exit string) (argsFile string) {
	t.Helper()
	dir := t.TempDir()
	argsFile = filepath.Join(dir, "args")
	script := "#!/bin/sh\necho \"$@\" > " + argsFile + "\nexit " + exit + "\n"
	if err := os.WriteFile(filepath.Join(dir, "ffmpeg"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return argsFile
}

func runStream(t *testing.T, d map[string]any) error {
	t.Helper()
	config.Config.SegmentPath = t.TempDir()
	d["streamID"] = uint64(1)
	d["streamVersion"] = "STREAM_VERSION_COMBINED"
	d["input"] = "rtmp://ingest/slug-1"
	notify := make(chan *protobuf.Notification, 8)
	return Stream(t.Context(), slog.New(slog.NewTextHandler(io.Discard, nil)), notify, d, testBroker())
}

func TestStreamDurationOnlyForScheduledSources(t *testing.T) {
	for _, self := range []bool{false, true} {
		args := fakeFFmpeg(t, "0")
		_ = runStream(t, map[string]any{"streamEnd": time.Now().Add(time.Hour), "selfStream": self})
		got, err := os.ReadFile(args)
		if err != nil {
			t.Fatal(err)
		}
		if has := strings.Contains(string(got), "-t "); has == self {
			t.Errorf("selfStream=%v: has -t = %v, args: %s", self, has, got)
		}
	}
}

func TestSelfStreamRetriesAfterScheduledEndWithinGrace(t *testing.T) {
	fakeFFmpeg(t, "1")
	err := runStream(t, map[string]any{
		"streamEnd":        time.Now().Add(-time.Minute),
		"selfStream":       true,
		selfStreamLastLive: time.Now().Add(-5 * time.Minute),
	})
	if err == nil || IsAbortingError(err) {
		t.Fatalf("want a retryable error while the publisher may come back, got %v", err)
	}
}

func TestSelfStreamEndsAfterScheduledEndOnceGraceIsOver(t *testing.T) {
	args := fakeFFmpeg(t, "1")
	err := runStream(t, map[string]any{
		"streamEnd":        time.Now().Add(-time.Hour),
		"selfStream":       true,
		selfStreamLastLive: time.Now().Add(-selfStreamGrace - time.Minute),
	})
	if err != nil {
		t.Fatalf("want nil, got %v", err)
	}
	if _, statErr := os.Stat(args); statErr == nil {
		t.Error("ffmpeg ran although the grace was over")
	}
}

func TestSelfStreamRetriesWhenPublisherDisconnectsBeforeEnd(t *testing.T) {
	fakeFFmpeg(t, "0")
	err := runStream(t, map[string]any{"streamEnd": time.Now().Add(time.Hour), "selfStream": true})
	if err == nil {
		t.Fatal("a clean ffmpeg exit before the end must be retried")
	}
}
