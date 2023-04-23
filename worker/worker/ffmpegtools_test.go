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

func TestGetCodec(t *testing.T) {
	codec, err := getCodec("testvid.mp4")
	if err != nil {
		t.Error(err)
		t.FailNow()
	}
	if codec != "h264" {
		t.Errorf("codec should be h264 but is %s", codec)
	}
}

func TestGetLevel(t *testing.T) {
	l, err := getLevel("testvid.mp4")
	if err != nil {
		t.Error(err)
		t.FailNow()
	}
	if l != "30" {
		t.Errorf("level should be 30 but is %s", l)
	}
}

func TestGetContainer(t *testing.T) {
	c, err := getContainer("testvid.mp4")
	if err != nil {
		t.Error(err)
		t.FailNow()
	}
	if c != "mov,mp4,m4a,3gp,3g2,mj2" {
		t.Errorf("codec should be mov,mp4,m4a,3gp,3g2,mj2 but is %s", c)
	}
}
