package actions

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/tum-dev/gocast/runner/pkg/metrics"
	"github.com/tum-dev/gocast/runner/protobuf"
)

func TestParseSilenceOutput(t *testing.T) {
	tests := []struct {
		name string
		out  string
		want []Silence
	}{
		{"none", "size=N/A time=00:10:00\n", nil},
		{"pairs", "[silencedetect @ 0x1] silence_start: 12.5\n[silencedetect @ 0x1] silence_end: 70.25 | silence_duration: 57.75\n", []Silence{{12, 70}}},
		{"negative start", "silence_start: -0.02\nsilence_end: 40 | silence_duration: 40\n", []Silence{{0, 40}}},
		{"end without start", "silence_end: 40 | silence_duration: 40\n", nil},
		{"unclosed trailing start", "silence_start: 1\nsilence_end: 40 | x\nsilence_start: 100\n", []Silence{{1, 40}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseSilenceOutput(tt.out); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMergeSilences(t *testing.T) {
	tests := []struct {
		name string
		in   []Silence
		want []Silence
	}{
		{"empty", nil, nil},
		{"single late stays", []Silence{{100, 200}}, []Silence{{100, 200}}},
		{"single early snaps to 0", []Silence{{10, 200}}, []Silence{{0, 200}}},
		{"short sound is bridged", []Silence{{100, 200}, {220, 400}}, []Silence{{100, 400}}},
		{"long sound splits", []Silence{{100, 200}, {300, 400}}, []Silence{{100, 200}, {300, 400}}},
		{"overlap does not underflow", []Silence{{100, 200}, {150, 180}}, []Silence{{100, 200}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := mergeSilences(tt.in); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMkSilenceMissingContext(t *testing.T) {
	err := MkSilence(context.Background(), testLogger(), make(chan *protobuf.Notification, 1), map[string]any{}, nil)
	if !IsAbortingError(err) {
		t.Fatalf("want aborting error, got %v", err)
	}
}

func TestMkSilenceFFmpeg(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not installed")
	}
	// 35 s of tone (boosted: the default sine is quieter than the -15 dB threshold), 40 s of silence, 35 s of tone; the lead-in is longer than the 30 s that would snap the start to 0.
	file := filepath.Join(t.TempDir(), "in.wav")
	cmd := exec.Command("ffmpeg", "-v", "error", "-f", "lavfi", "-i", "sine=frequency=440:duration=35,volume=4",
		"-f", "lavfi", "-i", "anullsrc=r=44100:cl=mono:d=40", "-f", "lavfi", "-i", "sine=frequency=440:duration=35,volume=4",
		"-filter_complex", "[0][1][2]concat=n=3:v=0:a=1", file)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("generate input: %v: %s", err, out)
	}
	if _, err := os.Stat(file); err != nil {
		t.Fatal(err)
	}

	notify := make(chan *protobuf.Notification, 1)
	d := map[string]any{"streamID": uint64(7), "streamVersion": "STREAM_VERSION_CAMERA", "vodPath": file}
	if err := MkSilence(context.Background(), testLogger(), notify, d, &metrics.Broker{}); err != nil {
		t.Fatal(err)
	}
	select {
	case n := <-notify:
		sd := n.GetSilenceDetected()
		if sd.GetStream().GetId() != 7 || len(sd.Starts) != 1 || sd.Starts[0] < 34 || sd.Starts[0] > 36 || sd.Ends[0] < 74 || sd.Ends[0] > 76 {
			t.Fatalf("unexpected notification: %v", sd)
		}
	default:
		t.Fatal("no notification sent")
	}
}
