package actions

import (
	"context"
	"errors"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tum-dev/gocast/runner/pkg/ffmpeg"
)

func TestConvertAudioVOD(t *testing.T) {
	requireAudioTools(t)
	input := makeAudioVODInput(t, "aac", "22")
	// Older recordings can expose a master playlist instead of a media playlist.
	master := "#EXTM3U\n#EXT-X-STREAM-INF:BANDWIDTH=200000\nplaylist.m3u8\n"
	if err := os.WriteFile(filepath.Join(filepath.Dir(input), "master.m3u8"), []byte(master), 0o600); err != nil {
		t.Fatal(err)
	}
	before := audioDirectoryContents(t, filepath.Dir(input))
	url := serveAudioVOD(t, input, nil)

	for name, playlist := range map[string]string{
		"local":       input,
		"HTTP":        url + "/playlist.m3u8",
		"HTTP master": url + "/master.m3u8",
	} {
		t.Run(name, func(t *testing.T) {
			audioDir := filepath.Join(t.TempDir(), "audio output")
			created, err := convertAudioVOD(audioTestContext(t), playlist, audioDir)
			if err != nil || !created {
				t.Fatalf("convertAudioVOD = (%v, %v), want (true, nil)", created, err)
			}
			assertAudioVOD(t, audioDir, 2, 22)
			assertNoLeftovers(t, filepath.Dir(audioDir), filepath.Base(audioDir))
		})
	}
	if got := audioDirectoryContents(t, filepath.Dir(input)); !reflect.DeepEqual(got, before) {
		t.Error("audio conversion changed the original recording")
	}
	decodeAudioTestMedia(t, input)
}

func TestConvertAudioVODTranscodes(t *testing.T) {
	requireAudioTools(t)
	input := makeAudioVODInput(t, "mp2", "2")
	audioDir := filepath.Join(t.TempDir(), "audio")
	created, err := convertAudioVOD(audioTestContext(t), input, audioDir)
	if err != nil || !created {
		t.Fatalf("convertAudioVOD = (%v, %v), want (true, nil)", created, err)
	}
	assertAudioVOD(t, audioDir, 1, 2)
}

func TestConvertAudioVODWithoutAudio(t *testing.T) {
	requireAudioTools(t)
	input := makeAudioVODInput(t, "", "2")
	for name, previous := range map[string]bool{"new output": false, "previous output": true} {
		t.Run(name, func(t *testing.T) {
			audioDir := filepath.Join(t.TempDir(), "audio")
			if previous {
				writePreviousAudio(t, audioDir)
			}
			created, err := convertAudioVOD(audioTestContext(t), input, audioDir)
			if err != nil || created {
				t.Fatalf("convertAudioVOD = (%v, %v), want (false, nil)", created, err)
			}
			if previous {
				assertPreviousAudio(t, audioDir)
			} else if _, err := os.Stat(audioDir); !os.IsNotExist(err) {
				t.Errorf("output exists for input without audio: %v", err)
			}
			assertNoLeftovers(t, filepath.Dir(audioDir), filepath.Base(audioDir))
		})
	}
}

func TestConvertAudioVODFailure(t *testing.T) {
	requireAudioTools(t)
	input := makeAudioVODInput(t, "aac", "2")
	t.Run("unreadable input", func(t *testing.T) {
		audioDir := filepath.Join(t.TempDir(), "audio")
		writePreviousAudio(t, audioDir)
		playlist := filepath.Join(t.TempDir(), "missing.m3u8")
		if created, err := convertAudioVOD(audioTestContext(t), playlist, audioDir); err == nil || created {
			t.Fatalf("convertAudioVOD = (%v, %v), want (false, error)", created, err)
		}
		assertPreviousAudio(t, audioDir)
		assertNoLeftovers(t, filepath.Dir(audioDir), filepath.Base(audioDir))
	})
	t.Run("output failure", func(t *testing.T) {
		parent := filepath.Join(t.TempDir(), "file")
		if err := os.WriteFile(parent, []byte("keep"), 0o600); err != nil {
			t.Fatal(err)
		}
		if created, err := convertAudioVOD(audioTestContext(t), input, filepath.Join(parent, "audio")); err == nil || created {
			t.Fatalf("convertAudioVOD = (%v, %v), want (false, error)", created, err)
		}
		if b, err := os.ReadFile(parent); err != nil || string(b) != "keep" {
			t.Errorf("output failure changed the existing file: %q, %v", b, err)
		}
	})
	t.Run("conversion failure", func(t *testing.T) {
		url := serveAudioVOD(t, input, func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "unavailable", http.StatusInternalServerError)
		})
		audioDir := filepath.Join(t.TempDir(), "audio")
		writePreviousAudio(t, audioDir)
		created, err := convertAudioVOD(audioTestContext(t), url+"/playlist.m3u8", audioDir)
		if err == nil || created {
			t.Fatalf("convertAudioVOD = (%v, %v), want (false, error)", created, err)
		}
		if !strings.HasPrefix(err.Error(), "convert audio VOD:") {
			t.Fatalf("input failed before reaching conversion: %v", err)
		}
		assertPreviousAudio(t, audioDir)
		assertNoLeftovers(t, filepath.Dir(audioDir), filepath.Base(audioDir))
	})
	decodeAudioTestMedia(t, input)
}

func TestConvertAudioVODReplacesPreviousOutput(t *testing.T) {
	requireAudioTools(t)
	input := makeAudioVODInput(t, "aac", "2")
	audioDir := filepath.Join(t.TempDir(), "audio")
	writePreviousAudio(t, audioDir)
	ctx := audioTestContext(t)
	for range 2 {
		created, err := convertAudioVOD(ctx, input, audioDir+string(filepath.Separator))
		if err != nil || !created {
			t.Fatalf("convertAudioVOD = (%v, %v), want (true, nil)", created, err)
		}
		assertAudioVOD(t, audioDir, 1, 2)
		if _, err := os.Stat(filepath.Join(audioDir, "99999.ts")); !os.IsNotExist(err) {
			t.Errorf("stale segment was not removed: %v", err)
		}
		assertNoLeftovers(t, filepath.Dir(audioDir), filepath.Base(audioDir))
	}
}

func TestAudioVODLog(t *testing.T) {
	warning := "[hls @ 0x1] Segment 1 of playlist 0 failed too many times, skipping\n"
	for split := 0; split <= len(warning); split++ {
		var log audioVODLog
		_, _ = log.Write([]byte(warning[:split]))
		_, _ = log.Write([]byte(warning[split:]))
		// Later diagnostics must neither hide the skipped segment nor grow the log
		// without bound. The warning may straddle any two writes from the process.
		_, _ = log.Write([]byte(strings.Repeat("x", 2*tailBufferSize)))
		if !log.skippedSegment || len(log.String()) > tailBufferSize {
			t.Fatalf("split %d: skipped=%v, diagnostic length=%d", split, log.skippedSegment, len(log.String()))
		}
	}
	var log audioVODLog
	_, _ = log.Write([]byte("[mpegts @ 0x1] Estimating duration from bitrate, this may be inaccurate\n"))
	if log.skippedSegment {
		t.Fatal("an unrelated warning was treated as a skipped segment")
	}
}

func TestConvertAudioVODMissingSegment(t *testing.T) {
	requireAudioTools(t)
	input := makeAudioVODInput(t, "aac", "45")
	// FFmpeg can skip an unreadable middle segment and still exit successfully.
	// Such audio loses part of the lecture and must not be published as complete.
	if _, err := os.Stat(filepath.Join(filepath.Dir(input), "00002.ts")); err != nil {
		t.Fatalf("fixture must contain three segments: %v", err)
	}
	if err := os.Remove(filepath.Join(filepath.Dir(input), "00001.ts")); err != nil {
		t.Fatal(err)
	}
	audioDir := filepath.Join(t.TempDir(), "audio")
	if created, err := convertAudioVOD(audioTestContext(t), input, audioDir); err == nil || created {
		t.Fatalf("convertAudioVOD = (%v, %v), want (false, error)", created, err)
	}
	if _, err := os.Stat(audioDir); !os.IsNotExist(err) {
		t.Errorf("incomplete audio was published: %v", err)
	}
	assertNoLeftovers(t, filepath.Dir(audioDir), "")
}

func TestConvertAudioVODCancelled(t *testing.T) {
	requireAudioTools(t)
	t.Run("before probe", func(t *testing.T) {
		audioDir := filepath.Join(t.TempDir(), "audio")
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		created, err := convertAudioVOD(ctx, "unused.m3u8", audioDir)
		if created || !errors.Is(err, context.Canceled) {
			t.Fatalf("convertAudioVOD = (%v, %v), want (false, context.Canceled)", created, err)
		}
		assertNoLeftovers(t, filepath.Dir(audioDir), "")
	})
	t.Run("reading HTTP input", func(t *testing.T) {
		input := makeAudioVODInput(t, "aac", "2")
		ctx, cancel := context.WithCancel(audioTestContext(t))
		defer cancel()
		release := make(chan struct{})
		defer close(release)
		url := serveAudioVOD(t, input, func(_ http.ResponseWriter, r *http.Request) {
			cancel()
			// Hold the response open: cancellation must stop the waiting process.
			select {
			case <-r.Context().Done():
			case <-release:
			}
		})
		audioDir := filepath.Join(t.TempDir(), "audio")
		writePreviousAudio(t, audioDir)
		type result struct {
			created bool
			err     error
		}
		done := make(chan result, 1)
		go func() {
			created, err := convertAudioVOD(ctx, url+"/playlist.m3u8", audioDir)
			done <- result{created, err}
		}()
		select {
		case got := <-done:
			if got.created || !errors.Is(got.err, context.Canceled) {
				t.Fatalf("convertAudioVOD = (%v, %v), want (false, context.Canceled)", got.created, got.err)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("cancellation did not stop conversion")
		}
		assertPreviousAudio(t, audioDir)
		assertNoLeftovers(t, filepath.Dir(audioDir), filepath.Base(audioDir))
	})
}

func audioTestContext(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	t.Cleanup(cancel)
	return ctx
}

// serveAudioVOD optionally changes the playlist response after ffprobe has read it,
// so failures and cancellation can be exercised during conversion.
func serveAudioVOD(t *testing.T, input string, onConversion http.HandlerFunc) string {
	t.Helper()
	var requests atomic.Int32
	files := http.FileServer(http.Dir(filepath.Dir(input)))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if onConversion != nil && r.URL.Path == "/playlist.m3u8" && requests.Add(1) > 1 {
			onConversion(w, r)
			return
		}
		files.ServeHTTP(w, r)
	}))
	t.Cleanup(server.Close)
	return server.URL
}

func requireAudioTools(t *testing.T) {
	t.Helper()
	for _, bin := range []string{"ffmpeg", "ffprobe"} {
		if _, err := exec.LookPath(bin); err != nil {
			t.Skipf("%s not installed", bin)
		}
	}
}

// Start from completed video HLS, so audio generation needs neither an original
// upload nor a live-capture directory.
func makeAudioVODInput(t *testing.T, audioCodec, duration string) string {
	t.Helper()
	ctx := audioTestContext(t)
	dir := t.TempDir()
	upload := filepath.Join(dir, "upload.mkv")
	args := []string{"-v", "error", "-nostdin", "-y", "-f", "lavfi", "-i", "testsrc=size=64x64:rate=2"}
	if audioCodec != "" {
		args = append(args, "-f", "lavfi", "-i", "sine=frequency=440", "-c:a", audioCodec, "-b:a", "128k")
	}
	args = append(args, "-t", duration, "-c:v", "libx264", "-preset", "ultrafast", "-pix_fmt", "yuv420p", "-g", "4", upload)
	if out, err := exec.CommandContext(ctx, "ffmpeg", args...).CombinedOutput(); err != nil {
		t.Fatalf("generate video: %v: %s", err, out)
	}
	vodDir := filepath.Join(dir, "vod")
	if err := os.Mkdir(vodDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := convertStream(ctx, testLogger(), 1, upload, vodDir, "playlist.m3u8"); err != nil {
		t.Fatalf("create completed VOD: %v", err)
	}
	// convertStream's existing argument splitting cannot handle spaces. Renaming
	// afterwards exercises paths with spaces in the new helper independently of that.
	withSpaces := filepath.Join(dir, "existing video")
	if err := os.Rename(vodDir, withSpaces); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(upload); err != nil {
		t.Fatal(err)
	}
	return filepath.Join(withSpaces, "playlist.m3u8")
}

func assertAudioVOD(t *testing.T, audioDir string, minSegments int, duration float64) {
	t.Helper()
	playlist := filepath.Join(audioDir, "playlist.m3u8")
	b, err := os.ReadFile(playlist)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "#EXT-X-PLAYLIST-TYPE:VOD") || !strings.HasSuffix(strings.TrimSpace(string(b)), "#EXT-X-ENDLIST") {
		t.Errorf("not a complete VOD playlist:\n%s", b)
	}
	segments := 0
	for _, line := range strings.Split(string(b), "\n") {
		if line != "" && !strings.HasPrefix(line, "#") {
			segments++
		}
	}
	if segments < minSegments {
		t.Errorf("playlist has %d segments, want at least %d", segments, minSegments)
	}
	probe, err := ffmpeg.Probe(audioTestContext(t), playlist)
	if err != nil {
		t.Fatal(err)
	}
	if audio, video := probe.Codec("audio"), probe.Codec("video"); audio != "aac" || video != "" {
		t.Errorf("output codecs: audio=%q video=%q, want AAC without video", audio, video)
	}
	if got := probe.Duration(); math.Abs(got-duration) > 0.25 {
		t.Errorf("audio duration = %v, want approximately %v seconds", got, duration)
	}
	decodeAudioTestMedia(t, playlist)
}

func decodeAudioTestMedia(t *testing.T, playlist string) {
	t.Helper()
	if out, err := exec.CommandContext(audioTestContext(t), "ffmpeg", "-v", "error", "-nostdin", "-xerror", "-i", playlist, "-map", "0", "-f", "null", "-").CombinedOutput(); err != nil {
		t.Fatalf("decode %s: %v: %s", playlist, err, out)
	}
}

func audioDirectoryContents(t *testing.T, dir string) map[string]string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	contents := make(map[string]string)
	for _, entry := range entries {
		b, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		contents[entry.Name()] = string(b)
	}
	return contents
}

func writePreviousAudio(t *testing.T, dir string) {
	t.Helper()
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"playlist.m3u8", "99999.ts"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("previous"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func assertPreviousAudio(t *testing.T, dir string) {
	t.Helper()
	want := map[string]string{"playlist.m3u8": "previous", "99999.ts": "previous"}
	if got := audioDirectoryContents(t, dir); !reflect.DeepEqual(got, want) {
		t.Errorf("previous audio changed: %v", got)
	}
}
