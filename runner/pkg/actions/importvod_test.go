package actions

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/tum-dev/gocast/runner/config"
	"github.com/tum-dev/gocast/runner/pkg/metrics"
	"github.com/tum-dev/gocast/runner/protobuf"
)

// testBroker is shared because the broker registers its metrics globally and panics when
// built twice.
var testBroker = sync.OnceValue(func() *metrics.Broker { return metrics.NewBroker() })

func TestNeedsTranscode(t *testing.T) {
	cases := []struct {
		name                string
		video, audio, level string
		want                bool
	}{
		{"h264 aac", "h264", "aac", "40", false},
		{"h264 without audio", "h264", "", "40", false},
		{"level at the limit", "h264", "aac", "42", false},
		{"level too high", "h264", "aac", "51", true},
		{"unknown level", "h264", "aac", "", true},
		{"hevc", "hevc", "aac", "40", true},
		{"ac3 audio", "h264", "ac3", "40", true},
	}
	for _, c := range cases {
		if got := needsTranscode(c.video, c.audio, c.level); got != c.want {
			t.Errorf("%s: needsTranscode(%q, %q, %q) = %v, want %v", c.name, c.video, c.audio, c.level, got, c.want)
		}
	}
}

func TestImportArgs(t *testing.T) {
	// A hostile file name must stay one argument.
	in := "/up/a file; rm -rf -i x.mp4"
	for _, transcode := range []bool{false, true} {
		args := importArgs(in, "/vod/1/X", transcode)
		var inputs int
		for i, a := range args {
			if a == "-i" {
				inputs++
				if args[i+1] != in {
					t.Errorf("transcode=%v: input arg = %q, want %q", transcode, args[i+1], in)
				}
			}
		}
		if inputs != 1 {
			t.Errorf("transcode=%v: %d inputs in %v, want 1", transcode, inputs, args)
		}
		if got := strings.Join(args, " "); strings.Contains(got, "-c copy") == transcode {
			t.Errorf("transcode=%v: unexpected codec options in %q", transcode, got)
		}
		if last := args[len(args)-1]; last != "/vod/1/X/playlist.m3u8" {
			t.Errorf("transcode=%v: output = %q", transcode, last)
		}
	}
}

func TestImportVODMissingContext(t *testing.T) {
	cases := map[string]map[string]any{
		"no stream id":      {"streamVersion": "x", "uploadPath": "y"},
		"no stream version": {"streamID": uint64(1), "uploadPath": "y"},
		"no upload path":    {"streamID": uint64(1), "streamVersion": "x"},
		"empty upload path": {"streamID": uint64(1), "streamVersion": "x", "uploadPath": ""},
	}
	for name, data := range cases {
		err := ImportVOD(context.Background(), testLogger(), nil, data, nil)
		if err == nil || !IsAbortingError(err) {
			t.Errorf("ImportVOD(%s) = %v, want an aborting error", name, err)
		}
	}
}

// makeUpload renders a short clip with the given ffmpeg codec options.
func makeUpload(t *testing.T, name string, codecArgs ...string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	args := append([]string{"-y",
		"-f", "lavfi", "-i", "testsrc=duration=3:size=320x240:rate=5",
		"-f", "lavfi", "-i", "sine=duration=3",
	}, codecArgs...)
	if out, err := exec.Command("ffmpeg", append(args, path)...).CombinedOutput(); err != nil {
		t.Skipf("could not create test video: %v (%s)", err, out)
	}
	return path
}

func TestImportVOD(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not available")
	}
	cases := []struct {
		name  string
		file  string
		codec []string
	}{
		{"copy", "up.mp4", []string{"-c:v", "libx264", "-pix_fmt", "yuv420p", "-c:a", "aac"}},
		{"transcode", "up.mkv", []string{"-c:v", "mpeg4", "-c:a", "mp3"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			upload := makeUpload(t, c.file, c.codec...)
			storage := t.TempDir()
			config.Config.StoragePath = storage
			config.Config.EdgeServer = "http://edge"

			notify := make(chan *protobuf.Notification, 1)
			data := map[string]any{"streamID": uint64(7), "streamVersion": "STREAM_VERSION_PRESENTATION", "uploadPath": upload}
			if err := ImportVOD(context.Background(), testLogger(), notify, data, testBroker()); err != nil {
				t.Fatalf("ImportVOD = %v", err)
			}

			playlist := filepath.Join(storage, "7", "STREAM_VERSION_PRESENTATION", "playlist.m3u8")
			b, err := os.ReadFile(playlist)
			if err != nil {
				t.Fatalf("no playlist: %v", err)
			}
			if !strings.Contains(string(b), "#EXT-X-ENDLIST") || !strings.Contains(string(b), "00000.ts") {
				t.Errorf("playlist is not a complete VOD:\n%s", b)
			}
			if data["playlistURL"] != playlist {
				t.Errorf("playlistURL = %v, want %s", data["playlistURL"], playlist)
			}
			if _, err := os.Stat(upload); !os.IsNotExist(err) {
				t.Errorf("uploaded file still there: %v", err)
			}
			if _, err := os.Stat(filepath.Join(storage, "7", "STREAM_VERSION_PRESENTATION.importing")); err == nil {
				t.Error("work directory left behind")
			}

			n := <-notify
			ready := n.GetVodReady()
			if ready == nil {
				t.Fatalf("notification = %v, want VodReady", n)
			}
			if got, want := ready.GetUrl(), "http://edge/vod/7/STREAM_VERSION_PRESENTATION/playlist.m3u8"; got != want {
				t.Errorf("url = %q, want %q", got, want)
			}
			if ready.GetStream().GetId() != 7 || ready.GetStreamVersion() != protobuf.StreamVersion_STREAM_VERSION_PRESENTATION {
				t.Errorf("unexpected stream in %v", ready)
			}
		})
	}
}

// TestImportVODFailureKeepsPreviousVOD checks that a broken re-upload neither replaces
// nor breaks the existing VOD, and still cleans up the upload.
func TestImportVODFailureKeepsPreviousVOD(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not available")
	}
	storage := t.TempDir()
	config.Config.StoragePath = storage
	old := filepath.Join(storage, "7", "STREAM_VERSION_COMB")
	if err := os.MkdirAll(old, os.ModePerm); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(old, "playlist.m3u8"), []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	upload := filepath.Join(t.TempDir(), "broken.mp4")
	if err := os.WriteFile(upload, []byte("not a video"), 0o600); err != nil {
		t.Fatal(err)
	}

	notify := make(chan *protobuf.Notification, 1)
	data := map[string]any{"streamID": uint64(7), "streamVersion": "STREAM_VERSION_COMB", "uploadPath": upload}
	err := ImportVOD(context.Background(), testLogger(), notify, data, testBroker())
	if err == nil || !IsAbortingError(err) {
		t.Fatalf("ImportVOD = %v, want an aborting error", err)
	}
	if b, _ := os.ReadFile(filepath.Join(old, "playlist.m3u8")); string(b) != "old" {
		t.Errorf("previous VOD was touched: %q", b)
	}
	if _, ok := data["playlistURL"]; ok {
		t.Error("playlistURL set although the import failed")
	}
	if len(notify) != 0 {
		t.Error("notified although the import failed")
	}
	if _, err := os.Stat(upload); !os.IsNotExist(err) {
		t.Errorf("uploaded file still there: %v", err)
	}
}

// TestImportVODThenThumbnail runs the chain an upload runs: the thumbnail is made from
// the local playlist ImportVOD leaves behind, through the playlistURL it hands over.
func TestImportVODThenThumbnail(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not available")
	}
	upload := makeUpload(t, "up.mp4", "-c:v", "libx264", "-pix_fmt", "yuv420p", "-c:a", "aac")
	config.Config.StoragePath = t.TempDir()

	notify := make(chan *protobuf.Notification, 2)
	data := map[string]any{"streamID": uint64(7), "streamVersion": "STREAM_VERSION_COMBINED", "uploadPath": upload}
	for _, action := range []Action{ImportVOD, RegenerateThumb} {
		if err := action(context.Background(), testLogger(), notify, data, testBroker()); err != nil {
			t.Fatalf("action: %v", err)
		}
	}

	if n := <-notify; n.GetVodReady() == nil {
		t.Fatalf("first notification = %v, want VodReady", n)
	}
	thumb := (<-notify).GetThumbnailReady()
	if thumb == nil || len(thumb.GetThumbnail()) == 0 {
		t.Fatalf("no thumbnail: %v", thumb)
	}
	if thumb.GetStream().GetId() != 7 || thumb.GetStreamVersion() != protobuf.StreamVersion_STREAM_VERSION_COMBINED {
		t.Errorf("thumbnail for %v", thumb)
	}
}
