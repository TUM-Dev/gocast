package actions

import (
	"bytes"
	"context"
	"log/slog"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/tum-dev/gocast/runner/protobuf"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))
}

func TestMkSectionImagesMissingContext(t *testing.T) {
	cases := map[string]map[string]any{
		"no stream id":    {"playlistURL": "x", "sections": []SectionTimestamp{{}}},
		"no playlist url": {"streamID": uint64(1), "sections": []SectionTimestamp{{}}},
		"empty playlist":  {"streamID": uint64(1), "playlistURL": "", "sections": []SectionTimestamp{{}}},
		"no sections":     {"streamID": uint64(1), "playlistURL": "x"},
	}
	for name, data := range cases {
		err := MkSectionImages(context.Background(), testLogger(), nil, data, nil)
		if err == nil {
			t.Errorf("MkSectionImages(%s) = nil, want error", name)
			continue
		}
		if !IsAbortingError(err) {
			t.Errorf("MkSectionImages(%s) = %v, want an aborting error", name, err)
		}
	}
}

// TestMkSectionImagesNoSections checks that a request without any section is a no-op
// rather than an error - there is simply nothing to generate.
func TestMkSectionImagesNoSections(t *testing.T) {
	data := map[string]any{
		"streamID":    uint64(1),
		"playlistURL": "x",
		"sections":    []SectionTimestamp{},
	}
	if err := MkSectionImages(context.Background(), testLogger(), nil, data, nil); err != nil {
		t.Errorf("MkSectionImages with no sections = %v, want nil", err)
	}
}

// TestMkSectionImages runs the action against a real video and checks that it reports
// one jpeg per section.
func TestMkSectionImages(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not available")
	}
	video := makeTestVideo(t)

	notify := make(chan *protobuf.Notification, 1)
	data := map[string]any{
		"streamID":    uint64(7),
		"playlistURL": video,
		"sections": []SectionTimestamp{
			{SectionID: 1, Seconds: 0},
			{SectionID: 2, Seconds: 2},
		},
	}

	if err := MkSectionImages(context.Background(), testLogger(), notify, data, nil); err != nil {
		t.Fatalf("MkSectionImages: %v", err)
	}

	select {
	case n := <-notify:
		got := n.GetSectionImagesReady()
		if got.GetStream().GetId() != 7 {
			t.Errorf("stream id = %d, want 7", got.GetStream().GetId())
		}
		if len(got.GetImages()) != 2 {
			t.Fatalf("got %d images, want 2", len(got.GetImages()))
		}
		for i, image := range got.GetImages() {
			if image.GetSectionId() != uint64(i+1) {
				t.Errorf("image %d has section id %d, want %d", i, image.GetSectionId(), i+1)
			}
			// jpeg files start with the SOI marker 0xFFD8
			if b := image.GetImage(); len(b) < 2 || b[0] != 0xFF || b[1] != 0xD8 {
				t.Errorf("image %d is not a jpeg (%d bytes)", i, len(b))
			}
		}
	case <-time.After(time.Second):
		t.Fatal("no notification sent")
	}
}

// TestMkSectionImagesSkipsUnreadableSections checks that one bad timestamp does not
// cost the sections that can be generated.
func TestMkSectionImagesSkipsUnreadableSections(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not available")
	}
	video := makeTestVideo(t)

	notify := make(chan *protobuf.Notification, 1)
	data := map[string]any{
		"streamID":    uint64(7),
		"playlistURL": video,
		"sections": []SectionTimestamp{
			{SectionID: 1, Seconds: 0},
			{SectionID: 2, Hours: 5}, // past the end of the video
		},
	}

	if err := MkSectionImages(context.Background(), testLogger(), notify, data, nil); err != nil {
		t.Fatalf("MkSectionImages: %v", err)
	}

	n := <-notify
	images := n.GetSectionImagesReady().GetImages()
	if len(images) != 1 || images[0].GetSectionId() != 1 {
		t.Fatalf("got %d images %+v, want only section 1", len(images), images)
	}
}

// makeTestVideo renders a short test pattern and returns its path.
func makeTestVideo(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.mp4")
	cmd := exec.Command("ffmpeg", "-y",
		"-f", "lavfi", "-i", "testsrc=duration=4:size=320x240:rate=5",
		"-pix_fmt", "yuv420p",
		path)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Skipf("could not create test video: %v (%s)", err, out)
	}
	return path
}
