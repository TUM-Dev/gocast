package actions

import (
	"context"
	"encoding/binary"
	"math"
	"path/filepath"
	"testing"

	"github.com/tum-dev/gocast/runner/protobuf"
)

func samples(vals ...int16) []byte {
	b := make([]byte, 2*len(vals))
	for i, v := range vals {
		binary.LittleEndian.PutUint16(b[2*i:], uint16(v))
	}
	return b
}

func TestLevelByte(t *testing.T) {
	for _, tt := range []struct {
		mag  int
		want byte
	}{
		{0, 0},
		{1, 0}, // -90 dBFS, below the floor
		{32768, 255},
		{16384, 229}, // -6 dBFS
		{33, 0},      // just under -60 dBFS
	} {
		if got := levelByte(tt.mag); got != tt.want {
			t.Errorf("levelByte(%d) = %d, want %d", tt.mag, got, tt.want)
		}
	}
	if levelByte(1000) >= levelByte(2000) {
		t.Error("louder must not map lower")
	}
}

func TestPeakWriter(t *testing.T) {
	w := newPeakWriter(4)
	// Negative samples count by magnitude, and the second bucket is only half full.
	w.Write(samples(0, -32768, 100, 0, 5, 6))
	got := w.Peaks()
	if len(got) != 2 || got[0] != 255 || got[1] != levelByte(6) {
		t.Errorf("peaks = %v, want [255 %d]", got, levelByte(6))
	}
}

func TestPeakWriterSplitsSamplesAcrossWrites(t *testing.T) {
	data := samples(0, 20000, -300, 0, 7, 1000)
	whole := newPeakWriter(3)
	whole.Write(data)

	// One byte at a time, which puts a chunk boundary inside every sample.
	split := newPeakWriter(3)
	for i := range data {
		split.Write(data[i : i+1])
	}
	if string(split.Peaks()) != string(whole.Peaks()) {
		t.Errorf("split writes gave %v, whole write %v", split.Peaks(), whole.Peaks())
	}
}

func TestMkWaveformMissingContext(t *testing.T) {
	notify := make(chan *protobuf.Notification, 1)
	for name, d := range map[string]map[string]any{
		"no stream":  {"sources": map[string]string{"STREAM_VERSION_COMBINED": "http://edge/a.m3u8"}},
		"no sources": {"streamID": uint64(1)},
		"unusable":   {"streamID": uint64(1), "sources": map[string]string{"STREAM_VERSION_UNSPECIFIED": "http://edge/a.m3u8"}},
	} {
		if err := MkWaveform(context.Background(), testLogger(), notify, d, nil); !IsAbortingError(err) {
			t.Errorf("%s: got %v, want an aborting error", name, err)
		}
	}
}

func TestMkWaveform(t *testing.T) {
	storage := importFixture(t) // a 3 s sine at 1/8 of full scale, about -18 dBFS
	notify := make(chan *protobuf.Notification, 1)
	d := map[string]any{
		"streamID": uint64(7),
		// The camera version can't be read: it is left out of the lengths, not reported as 0.
		"sources": map[string]string{
			"STREAM_VERSION_COMBINED": filepath.Join(storage, "7", "STREAM_VERSION_COMBINED", "playlist.m3u8"),
			"STREAM_VERSION_CAMERA":   filepath.Join(storage, "7", "STREAM_VERSION_CAMERA", "playlist.m3u8"),
		},
	}
	if err := MkWaveform(context.Background(), testLogger(), notify, d, nil); err != nil {
		t.Fatalf("MkWaveform = %v", err)
	}
	n := (<-notify).GetWaveformReady()
	if n == nil {
		t.Fatal("no WaveformReady")
	}
	if n.GetStream().GetId() != 7 || n.GetBucketSeconds() != 1 {
		t.Errorf("got stream %d, bucket %d", n.GetStream().GetId(), n.GetBucketSeconds())
	}
	if got := len(n.GetPeaks()); got < 3 || got > 4 {
		t.Errorf("%d peaks for 3 s of audio", got)
	}
	want := byte(math.Round((20*math.Log10(0.125) + 60) / 60 * 255))
	for i, p := range n.GetPeaks() {
		if math.Abs(float64(p)-float64(want)) > 12 {
			t.Errorf("peak %d = %d, want about %d", i, p, want)
		}
	}
	if len(n.GetDurations()) != 1 || n.GetDurations()[0].GetStreamVersion() != protobuf.StreamVersion_STREAM_VERSION_COMBINED ||
		math.Abs(n.GetDurations()[0].GetSeconds()-3) > 0.5 {
		t.Errorf("durations = %v, want only the combined one, 3 s", n.GetDurations())
	}
}
