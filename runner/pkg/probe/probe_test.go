package probe

import (
	"context"
	"testing"
)

func TestProbe(t *testing.T) {
	p, err := Probe(context.Background(), "../../testvid.mp4")
	if err != nil {
		t.Errorf("got error %v", err)
		t.FailNow()
	}
	if len(p.Streams) != 1 {
		t.Errorf("ffprobe len(streams), want: 1, got: %d", len(p.Streams))

	}
	if p.Streams[0].CodecName != "h264" {
		t.Errorf("ffprobe stream codec name, want: h264, got: %s", p.Streams[0].CodecName)
	}
	if duration, err := p.Streams[0].DurationFloat(); err != nil {
		t.Errorf("ffprobe get Duration: %v", err)
	} else if duration != 1 {
		t.Errorf("ffprobe stream duration, want: 1, got: %f", duration)
	}

}
