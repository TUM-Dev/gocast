package actions

import (
	"bytes"
	"context"
	"image"
	"image/jpeg"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/tum-dev/gocast/runner/pkg/metrics"
	"github.com/tum-dev/gocast/runner/protobuf"
)

func TestSpriteFilter(t *testing.T) {
	for _, tc := range []struct {
		duration float64
		tile     string
	}{
		{1, "tile=17x1"},
		{30 * 15, "tile=17x1"}, // 16 frames
		{30 * 16, "tile=17x1"}, // 17 frames, a full row
		{30 * 17, "tile=17x2"},
		{90 * 60, "tile=17x11"}, // a lecture: 181 frames
	} {
		if got := spriteFilter(tc.duration); !strings.HasSuffix(got, tc.tile) {
			t.Errorf("duration %v: %q does not end in %s", tc.duration, got, tc.tile)
		}
	}
}

func TestMkSpriteMissingContext(t *testing.T) {
	err := MkSprite(context.Background(), testLogger(), make(chan *protobuf.Notification, 1), map[string]any{}, nil)
	if !IsAbortingError(err) {
		t.Fatalf("want aborting error, got %v", err)
	}
}

// runSprite returns the notification of a sprite of input, nil if none was sent.
func runSprite(t *testing.T, input string) *protobuf.SpriteReadyNotification {
	t.Helper()
	notify := make(chan *protobuf.Notification, 1)
	d := map[string]any{"streamID": uint64(7), "streamVersion": "STREAM_VERSION_CAMERA", "vodPath": input}
	if err := MkSprite(t.Context(), testLogger(), notify, d, &metrics.Broker{}); err != nil {
		t.Fatal(err)
	}
	select {
	case n := <-notify:
		return n.GetSpriteReady()
	default:
		return nil
	}
}

func requireFFmpeg(t *testing.T) {
	t.Helper()
	for _, bin := range []string{"ffmpeg", "ffprobe"} {
		if _, err := exec.LookPath(bin); err != nil {
			t.Skipf("%s not installed", bin)
		}
	}
}

// The player cuts frames out of the sheet by a fixed size, so the sheet must have exactly that
// layout, also for a video that isn't 16:9.
func TestMkSpriteLayout(t *testing.T) {
	requireFFmpeg(t)
	for name, size := range map[string]string{"16:9": "320x180", "4:3": "320x240"} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			playlist := filepath.Join(dir, "playlist.m3u8")
			// 10 minutes make 21 frames, so 2 rows.
			out, err := exec.Command("ffmpeg", "-v", "error", "-y", "-f", "lavfi", "-i", "testsrc=size="+size+":rate=1",
				"-t", "600", "-c:v", "libx264", "-preset", "ultrafast", "-g", "4",
				"-f", "hls", "-hls_time", "4", "-hls_playlist_type", "vod",
				"-hls_segment_filename", filepath.Join(dir, "%05d.ts"), playlist).CombinedOutput()
			if err != nil {
				t.Fatalf("recording: %v: %s", err, out)
			}

			n := runSprite(t, playlist)
			if n == nil {
				t.Fatal("no notification sent")
			}
			if n.GetStream().GetId() != 7 || n.GetStreamVersion() != protobuf.StreamVersion_STREAM_VERSION_CAMERA || n.GetIntervalSeconds() != spriteIntervalSeconds {
				t.Errorf("unexpected notification: stream %d version %v interval %d", n.GetStream().GetId(), n.GetStreamVersion(), n.GetIntervalSeconds())
			}
			img, err := jpeg.DecodeConfig(bytes.NewReader(n.GetSprite()))
			if err != nil {
				t.Fatalf("not a JPEG: %v", err)
			}
			if want := spriteFrameWidth * spriteColumns; img.Width != want {
				t.Errorf("width %d, want %d", img.Width, want)
			}
			if want := spriteFrameHeight * 2; img.Height != want {
				t.Errorf("height %d, want %d", img.Height, want)
			}
		})
	}
}

// cellIsBlack tells whether the cell a hover on the given second shows is empty.
func cellIsBlack(t *testing.T, sprite []byte, second int) bool {
	t.Helper()
	img, err := jpeg.Decode(bytes.NewReader(sprite))
	if err != nil {
		t.Fatalf("not a JPEG: %v", err)
	}
	cell := second / spriteIntervalSeconds
	x := cell%spriteColumns*spriteFrameWidth + spriteFrameWidth/2
	y := cell/spriteColumns*spriteFrameHeight + spriteFrameHeight/2
	if !image.Pt(x, y).In(img.Bounds()) {
		t.Fatalf("cell %d is outside the sheet %v", cell, img.Bounds())
	}
	r, g, b, _ := img.At(x, y).RGBA()
	return r>>8 < 16 && g>>8 < 16 && b>>8 < 16
}

// 905 s end 5 s into their last interval, which fps drops by default.
func TestMkSpriteKeepsTheLastFrame(t *testing.T) {
	requireFFmpeg(t)
	file := filepath.Join(t.TempDir(), "in.mp4")
	out, err := exec.Command("ffmpeg", "-v", "error", "-y", "-f", "lavfi", "-i", "color=white:size=320x180:rate=1",
		"-t", "905", "-c:v", "libx264", "-preset", "ultrafast", "-g", "4", file).CombinedOutput()
	if err != nil {
		t.Fatalf("recording: %v: %s", err, out)
	}
	n := runSprite(t, file)
	if n == nil {
		t.Fatal("no notification sent")
	}
	if cellIsBlack(t, n.GetSprite(), 904) {
		t.Error("the last cell is black")
	}
}

// A self-stream whose publisher reconnected is one playlist of pieces with timestamps
// restarting near zero, see createVideoThumbnail, possibly at another resolution.
func TestMkSpriteAcrossReconnects(t *testing.T) {
	requireFFmpeg(t)
	dir := t.TempDir()
	playlist := filepath.Join(dir, "playlist.m3u8")
	for _, size := range []string{"320x180", "640x360", "320x180"} {
		out, err := exec.Command("ffmpeg", "-v", "error", "-y", "-f", "lavfi", "-i", "color=white:size="+size+":rate=1",
			"-t", strconv.Itoa(120), "-c:v", "libx264", "-preset", "ultrafast", "-g", "4",
			"-f", "hls", "-hls_time", "4", "-hls_playlist_type", "event", "-hls_flags", "append_list",
			"-hls_segment_filename", filepath.Join(dir, "%05d.ts"), playlist).CombinedOutput()
		if err != nil {
			t.Fatalf("recording: %v: %s", err, out)
		}
	}
	n := runSprite(t, playlist)
	if n == nil {
		t.Fatal("no notification sent")
	}
	if cellIsBlack(t, n.GetSprite(), 300) {
		t.Error("the sheet ends at the reconnect")
	}
}

// A sprite that can't be made must not fail a job whose VoD is done.
func TestMkSpriteFailureIsNotFatal(t *testing.T) {
	requireFFmpeg(t)
	if n := runSprite(t, filepath.Join(t.TempDir(), "missing.m3u8")); n != nil {
		t.Errorf("want no notification, got %v", n)
	}
}
