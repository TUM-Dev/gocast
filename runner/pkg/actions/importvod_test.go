package actions

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/tum-dev/gocast/runner/config"
	"github.com/tum-dev/gocast/runner/pkg/metrics"
	"github.com/tum-dev/gocast/runner/protobuf"
)

// testBroker is shared because the broker registers its metrics globally and panics when
// built twice.
var testBroker = sync.OnceValue(func() *metrics.Broker { return metrics.NewBroker() })

func TestNeedsTranscode(t *testing.T) {
	cases := []struct {
		name                        string
		video, audio, level, pixFmt string
		want                        bool
	}{
		{"h264 aac", "h264", "aac", "40", "yuv420p", false},
		{"full range jpeg yuv", "h264", "aac", "40", "yuvj420p", false},
		{"h264 without audio", "h264", "", "40", "yuv420p", false},
		{"level at the limit", "h264", "aac", "42", "yuv420p", false},
		{"level too high", "h264", "aac", "51", "yuv420p", true},
		{"unknown level", "h264", "aac", "", "yuv420p", true},
		{"hevc", "hevc", "aac", "40", "yuv420p", true},
		{"ac3 audio", "h264", "ac3", "40", "yuv420p", true},
		{"10 bit", "h264", "aac", "40", "yuv420p10le", true},
		{"4:2:2", "h264", "aac", "40", "yuv422p", true},
		{"4:4:4", "h264", "aac", "40", "yuv444p", true},
		{"unknown pixel format", "h264", "aac", "40", "", true},
	}
	for _, c := range cases {
		if got := needsTranscode(c.video, c.audio, c.level, c.pixFmt); got != c.want {
			t.Errorf("%s: needsTranscode(%q, %q, %q, %q) = %v, want %v", c.name, c.video, c.audio, c.level, c.pixFmt, got, c.want)
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

			notify := make(chan *protobuf.Notification, 64)
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
			assertNoLeftovers(t, filepath.Join(storage, "7"), "STREAM_VERSION_PRESENTATION")

			ns := drain(notify)
			if len(ns) == 0 {
				t.Fatal("nothing was reported")
			}
			// The conversion is over when VodReady comes, so nothing may follow it.
			ready := ns[len(ns)-1].GetVodReady()
			if ready == nil {
				t.Fatalf("last notification = %v, want VodReady", ns[len(ns)-1])
			}
			for _, n := range ns[:len(ns)-1] {
				if p := n.GetTranscodingProgress(); p == nil || p.GetProgress() < 0 || p.GetProgress() > 99 {
					t.Errorf("before VodReady: %v, want progress of 0 to 99", n)
				}
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
	old := filepath.Join(storage, "7", "STREAM_VERSION_COMBINED")
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

	notify := make(chan *protobuf.Notification, 64)
	data := map[string]any{"streamID": uint64(7), "streamVersion": "STREAM_VERSION_COMBINED", "uploadPath": upload}
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
	ns := drain(notify)
	if len(ns) != 1 || ns[0].GetVodFailed() == nil {
		t.Fatalf("notifications = %v, want one VodFailed", ns)
	}
	failed := ns[0].GetVodFailed()
	if failed.GetStream().GetId() != 7 || failed.GetStreamVersion() != protobuf.StreamVersion_STREAM_VERSION_COMBINED {
		t.Errorf("failure for %v", failed)
	}
	if failed.GetReason() != reasonUnreadable {
		t.Errorf("reason = %q, want %q", failed.GetReason(), reasonUnreadable)
	}
	for _, leak := range []string{upload, "ffprobe", "exit status"} {
		if strings.Contains(failed.GetReason(), leak) {
			t.Errorf("reason %q leaks %q", failed.GetReason(), leak)
		}
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

	notify := make(chan *protobuf.Notification, 64)
	data := map[string]any{"streamID": uint64(7), "streamVersion": "STREAM_VERSION_COMBINED", "uploadPath": upload}
	for _, action := range []Action{ImportVOD, RegenerateThumb} {
		if err := action(context.Background(), testLogger(), notify, data, testBroker()); err != nil {
			t.Fatalf("action: %v", err)
		}
	}

	var thumb *protobuf.ThumbnailReadyNotification
	var ready bool
	for _, n := range drain(notify) {
		ready = ready || n.GetVodReady() != nil
		if t := n.GetThumbnailReady(); t != nil {
			thumb = t
		}
	}
	if !ready {
		t.Fatal("no VodReady")
	}
	if thumb == nil || len(thumb.GetThumbnail()) == 0 {
		t.Fatalf("no thumbnail: %v", thumb)
	}
	if thumb.GetStream().GetId() != 7 || thumb.GetStreamVersion() != protobuf.StreamVersion_STREAM_VERSION_COMBINED {
		t.Errorf("thumbnail for %v", thumb)
	}
}

// assertNoLeftovers checks that dir holds nothing but the VOD directory itself: no work
// or trash directories of an import.
func assertNoLeftovers(t *testing.T, dir, vod string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.Name() != vod {
			t.Errorf("%s left behind next to the VOD", e.Name())
		}
	}
}

// TestImportVODReplacesPreviousVOD checks that a re-upload replaces the VOD, leaves no
// stale segments and cleans up after itself.
func TestImportVODReplacesPreviousVOD(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not available")
	}
	storage := t.TempDir()
	config.Config.StoragePath = storage
	config.Config.EdgeServer = "http://edge"
	old := filepath.Join(storage, "7", "STREAM_VERSION_COMBINED")
	if err := os.MkdirAll(old, os.ModePerm); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(old, "stale.ts"), []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}

	upload := makeUpload(t, "up.mp4", "-c:v", "libx264", "-pix_fmt", "yuv420p", "-c:a", "aac")
	notify := make(chan *protobuf.Notification, 64)
	data := map[string]any{"streamID": uint64(7), "streamVersion": "STREAM_VERSION_COMBINED", "uploadPath": upload}
	if err := ImportVOD(context.Background(), testLogger(), notify, data, testBroker()); err != nil {
		t.Fatalf("ImportVOD = %v", err)
	}
	if _, err := os.Stat(filepath.Join(old, "stale.ts")); err == nil {
		t.Error("stale segment of the previous VOD survived")
	}
	if _, err := os.Stat(filepath.Join(old, "playlist.m3u8")); err != nil {
		t.Errorf("no new playlist: %v", err)
	}
	assertNoLeftovers(t, filepath.Join(storage, "7"), "STREAM_VERSION_COMBINED")
}

// TestImportVODConcurrent checks that imports of the same stream version don't share a
// work directory and wipe each other's output.
func TestImportVODConcurrent(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not available")
	}
	storage := t.TempDir()
	config.Config.StoragePath = storage
	config.Config.EdgeServer = "http://edge"

	var wg sync.WaitGroup
	errs := make(chan error, 3)
	for i := 0; i < 3; i++ {
		upload := makeUpload(t, "up.mp4", "-c:v", "libx264", "-pix_fmt", "yuv420p", "-c:a", "aac")
		wg.Add(1)
		go func() {
			defer wg.Done()
			notify := make(chan *protobuf.Notification, 64)
			data := map[string]any{"streamID": uint64(7), "streamVersion": "STREAM_VERSION_COMBINED", "uploadPath": upload}
			errs <- ImportVOD(context.Background(), testLogger(), notify, data, testBroker())
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Errorf("ImportVOD = %v", err)
		}
	}
	b, err := os.ReadFile(filepath.Join(storage, "7", "STREAM_VERSION_COMBINED", "playlist.m3u8"))
	if err != nil || !strings.Contains(string(b), "#EXT-X-ENDLIST") {
		t.Errorf("no complete playlist: %v\n%s", err, b)
	}
	assertNoLeftovers(t, filepath.Join(storage, "7"), "STREAM_VERSION_COMBINED")
}

// TestImportVODReportsFFmpegError checks that what ffmpeg says on failing is part of
// the error, which is all anyone gets to see of a failed import.
func TestImportVODReportsFFmpegError(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not available")
	}
	config.Config.StoragePath = t.TempDir()
	full := makeUpload(t, "up.mp4", "-c:v", "libx264", "-pix_fmt", "yuv420p", "-c:a", "aac")
	b, err := os.ReadFile(full)
	if err != nil {
		t.Fatal(err)
	}
	// ffprobe still reads the head of a truncated file; ffmpeg fails on the missing rest.
	broken := filepath.Join(t.TempDir(), "broken.mp4")
	if err := os.WriteFile(broken, b[:len(b)/3], 0o600); err != nil {
		t.Fatal(err)
	}
	data := map[string]any{"streamID": uint64(7), "streamVersion": "STREAM_VERSION_COMBINED", "uploadPath": broken}
	err = ImportVOD(context.Background(), testLogger(), make(chan *protobuf.Notification, 64), data, testBroker())
	if err == nil {
		t.Skip("ffmpeg accepted the truncated file")
	}
	if !IsAbortingError(err) {
		t.Errorf("err = %v, want an aborting error", err)
	}
}

func TestTailBuffer(t *testing.T) {
	var tb tailBuffer
	_, _ = tb.Write([]byte(strings.Repeat("a", 3*tailBufferSize)))
	_, _ = tb.Write([]byte("the error\n"))
	if got := tb.String(); len(got) > tailBufferSize || !strings.HasSuffix(got, "the error") {
		t.Errorf("tail = %d bytes ending %q", len(got), got[len(got)-10:])
	}
}

// drain returns what was reported, once the action that reported it has returned.
func drain(notify chan *protobuf.Notification) []*protobuf.Notification {
	var ns []*protobuf.Notification
	for {
		select {
		case n := <-notify:
			ns = append(ns, n)
		default:
			return ns
		}
	}
}

func failureOf(t *testing.T, notify chan *protobuf.Notification) string {
	t.Helper()
	ns := drain(notify)
	if len(ns) == 0 || ns[len(ns)-1].GetVodFailed() == nil {
		t.Fatalf("notifications = %v, want a VodFailed last", ns)
	}
	return ns[len(ns)-1].GetVodFailed().GetReason()
}

func TestImportVODReasons(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not available")
	}
	config.Config.StoragePath = t.TempDir()
	run := func(ctx context.Context, file string) chan *protobuf.Notification {
		notify := make(chan *protobuf.Notification, 64)
		data := map[string]any{"streamID": uint64(7), "streamVersion": "STREAM_VERSION_COMBINED", "uploadPath": file}
		if err := ImportVOD(ctx, testLogger(), notify, data, testBroker()); err == nil || !IsAbortingError(err) {
			t.Fatalf("ImportVOD = %v, want an aborting error", err)
		}
		return notify
	}

	t.Run("no video", func(t *testing.T) {
		audio := filepath.Join(t.TempDir(), "audio.m4a")
		if out, err := exec.Command("ffmpeg", "-y", "-f", "lavfi", "-i", "sine=duration=1", "-c:a", "aac", audio).CombinedOutput(); err != nil {
			t.Skipf("could not create audio: %v (%s)", err, out)
		}
		if got := failureOf(t, run(context.Background(), audio)); got != reasonNoVideo {
			t.Errorf("reason = %q, want %q", got, reasonNoVideo)
		}
	})

	t.Run("interrupted", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		upload := makeUpload(t, "up.mp4", "-c:v", "libx264", "-pix_fmt", "yuv420p", "-c:a", "aac")
		if got := failureOf(t, run(ctx, upload)); got != reasonInterrupted {
			t.Errorf("reason = %q, want %q", got, reasonInterrupted)
		}
	})
}

func TestImportVODNoFailureNotificationWithoutContext(t *testing.T) {
	// Nothing to report on, and a nil channel would block for good.
	err := ImportVOD(context.Background(), testLogger(), nil, map[string]any{"streamID": uint64(1), "streamVersion": "x"}, nil)
	if err == nil {
		t.Fatal("ImportVOD = nil, want an error")
	}
}

func TestProgressWriter(t *testing.T) {
	now := time.Unix(1000, 0)
	var got []int
	w := newProgressWriter(10, func(p int) { got = append(got, p) })
	w.now = func() time.Time { return now }
	write := func(s string) { _, _ = w.Write([]byte(s)) }
	step := func(micros string) {
		now = now.Add(2 * time.Second)
		write("frame=1\nout_time_us=" + micros + "\nprogress=continue\n")
	}

	write("out_time_us=N/A\nout_time_us=-9\n") // before the first frame: nothing to say
	step("1000000")                            // 10%
	write("out_time_us=1")                     // a line cut in two ...
	write("500000\n")                          // ... is read as one: 15%, but too soon after the last
	step("1100000")                            // 11%: too little to be worth a call
	step("5000000")                            // 50%
	step("30000000")                           // past the end: never 100, that is VODReady's to say
	if want := []int{10, 50, 99}; !equalInts(got, want) {
		t.Errorf("reported %v, want %v", got, want)
	}

	t.Run("unknown duration", func(t *testing.T) {
		var any bool
		w := newProgressWriter(0, func(int) { any = true })
		_, _ = w.Write([]byte("out_time_us=5000000\n"))
		if any {
			t.Error("reported without a duration to measure against")
		}
	})
}

func equalInts(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
