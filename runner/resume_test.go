package runner

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/tum-dev/gocast/runner/pkg/actions"
)

// What MarkVoDPending writes at the end of a stream is what a restarted runner resumes.
func TestPendingVoDIsResumed(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "81821", "STREAM_VERSION_PRESENTATION")
	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		t.Fatal(err)
	}
	d := map[string]any{
		"streamID":      uint64(81821),
		"streamVersion": "STREAM_VERSION_PRESENTATION",
		"recordingDir":  dir,
		"detectSilence": true,
	}
	if err := actions.MarkVoDPending(context.Background(), nil, nil, d, nil); err != nil {
		t.Fatal(err)
	}

	pending, errs := findPendingVoDs(root)
	if len(errs) > 0 {
		t.Fatal(errs)
	}
	want := []pendingVoD{{
		PendingVoD:   actions.PendingVoD{StreamID: 81821, StreamVersion: "STREAM_VERSION_PRESENTATION", DetectSilence: true},
		recordingDir: dir,
	}}
	if !reflect.DeepEqual(pending, want) {
		t.Errorf("pending = %+v, want %+v", pending, want)
	}
}

// A recording without the marker is one whose stream was cut off, was discarded, or whose
// VoD is written already. None of them may be made into a VoD on start.
func TestRecordingWithoutMarkerIsNotResumed(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "1", "STREAM_VERSION_COMBINED")
	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "playlist.m3u8"), []byte("#EXTM3U\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if pending, errs := findPendingVoDs(root); len(pending) > 0 || len(errs) > 0 {
		t.Errorf("pending = %+v, errs = %v, want neither", pending, errs)
	}
}

func TestBrokenMarkerIsReportedNotResumed(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "1", "STREAM_VERSION_COMBINED")
	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, actions.VoDPendingFile), []byte(`{"streamID":1,"streamVersion":"nope"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	pending, errs := findPendingVoDs(root)
	if len(pending) > 0 || len(errs) != 1 {
		t.Errorf("pending = %+v, errs = %v, want one error", pending, errs)
	}
}

// The marker must be written before MkVOD, which removes it, and only in the VoD phase,
// which a discarded recording skips.
func TestVoDActionsMarkPendingFirst(t *testing.T) {
	for _, silence := range []bool{false, true} {
		got := vodActions(silence)
		if reflect.ValueOf(got[0]).Pointer() != reflect.ValueOf(actions.MarkVoDPending).Pointer() ||
			reflect.ValueOf(got[1]).Pointer() != reflect.ValueOf(actions.MkVOD).Pointer() {
			t.Errorf("detectSilence=%v: want MarkVoDPending, then MkVOD", silence)
		}
		last := reflect.ValueOf(got[len(got)-1]).Pointer() == reflect.ValueOf(actions.MkSilence).Pointer()
		if last != silence {
			t.Errorf("detectSilence=%v: MkSilence last = %v", silence, last)
		}
	}
}

// A marker moved into another recording's directory must not make a VoD of that recording
// for the stream it names.
func TestMarkerInAnotherDirectoryIsReportedNotResumed(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "123", "STREAM_VERSION_CAMERA")
	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, actions.VoDPendingFile), []byte(`{"streamID":456,"streamVersion":"STREAM_VERSION_CAMERA"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	pending, errs := findPendingVoDs(root)
	if len(pending) > 0 || len(errs) != 1 {
		t.Errorf("pending = %+v, errs = %v, want one error", pending, errs)
	}
}

// Drain ends the wait of VoD jobs for the storage, and a second one must not panic.
func TestDrainClosesDrained(t *testing.T) {
	r := newTestRunner()
	drained := r.drainedCh()
	r.Drain(context.Background())
	r.Drain(context.Background())
	select {
	case <-drained:
	default:
		t.Error("drained is still open after Drain")
	}
}
