package actions

import (
	"context"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
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
	server := httptest.NewServer(http.FileServer(http.Dir(filepath.Dir(input))))
	defer server.Close()

	for name, playlist := range map[string]string{
		"local":       input,
		"HTTP":        server.URL + "/playlist.m3u8",
		"HTTP master": server.URL + "/master.m3u8",
	} {
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
			defer cancel()
			audioDir := filepath.Join(t.TempDir(), "audio output")
			created, err := convertAudioVOD(ctx, playlist, audioDir)
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
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	created, err := convertAudioVOD(ctx, input, audioDir)
	if err != nil || !created {
		t.Fatalf("convertAudioVOD = (%v, %v), want (true, nil)", created, err)
	}
	assertAudioVOD(t, audioDir, 1, 2)
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
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
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
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	probe, err := ffmpeg.Probe(ctx, playlist)
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
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	if out, err := exec.CommandContext(ctx, "ffmpeg", "-v", "error", "-nostdin", "-xerror", "-i", playlist, "-map", "0", "-f", "null", "-").CombinedOutput(); err != nil {
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
