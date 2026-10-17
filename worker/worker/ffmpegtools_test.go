package worker

import "testing"

func TestGetDuration(t *testing.T) {
	duration, err := getDuration("testvid.mp4")
	if err != nil {
		t.Error(err)
		t.FailNow()
	}
	if duration != 1.000000 {
		t.Errorf("duration should be 1.000000 but is %f", duration)
	}
}
