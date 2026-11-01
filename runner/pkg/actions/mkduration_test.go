package actions

import (
	"context"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/tum-dev/gocast/runner/protobuf"
)

func TestMkDurationMissingContext(t *testing.T) {
	err := MkDuration(context.Background(), testLogger(), make(chan *protobuf.Notification, 1), map[string]any{}, nil)
	if !IsAbortingError(err) {
		t.Fatalf("want aborting error, got %v", err)
	}
}

func TestMkDurationFFmpeg(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not installed")
	}
	file := filepath.Join(t.TempDir(), "in.wav")
	if out, err := exec.Command("ffmpeg", "-v", "error", "-f", "lavfi", "-i", "sine=frequency=440:duration=12.7", file).CombinedOutput(); err != nil {
		t.Fatalf("generate input: %v: %s", err, out)
	}

	notify := make(chan *protobuf.Notification, 1)
	d := map[string]any{"streamID": uint64(7), "streamVersion": "STREAM_VERSION_CAMERA", "vodPath": file}
	if err := MkDuration(context.Background(), testLogger(), notify, d, nil); err != nil {
		t.Fatal(err)
	}
	select {
	case n := <-notify:
		dp := n.GetDurationProbed()
		if dp.GetStream().GetId() != 7 || dp.GetStreamVersion() != protobuf.StreamVersion_STREAM_VERSION_CAMERA || dp.GetDurationSeconds() != 12 {
			t.Fatalf("unexpected notification: %v", dp)
		}
	default:
		t.Fatal("no notification sent")
	}
}

// A probe that fails reports nothing, so that the duration a stream has is kept, and does
// not fail a job whose VoD is already published.
func TestMkDurationFailedProbeSendsNothing(t *testing.T) {
	if _, err := exec.LookPath("ffprobe"); err != nil {
		t.Skip("ffprobe not installed")
	}
	notify := make(chan *protobuf.Notification, 1)
	d := map[string]any{"streamID": uint64(7), "streamVersion": "STREAM_VERSION_COMBINED", "vodPath": filepath.Join(t.TempDir(), "missing.m3u8")}
	if err := MkDuration(context.Background(), testLogger(), notify, d, nil); err != nil {
		t.Fatalf("a failed probe must not fail the job: %v", err)
	}
	select {
	case n := <-notify:
		t.Fatalf("unexpected notification: %v", n)
	default:
	}
}
