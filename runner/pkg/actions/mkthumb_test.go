package actions

import (
	"bytes"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
)

// A self-stream whose publisher reconnected is one playlist of several pieces with
// timestamps restarting near zero. The thumbnail is taken at half the playlist's duration,
// which then lies beyond every single piece.
func TestCreateVideoThumbnailAcrossReconnects(t *testing.T) {
	for _, bin := range []string{"ffmpeg", "ffprobe"} {
		if _, err := exec.LookPath(bin); err != nil {
			t.Skipf("%s not installed", bin)
		}
	}
	dir := t.TempDir()
	playlist := filepath.Join(dir, "playlist.m3u8")
	// the way Stream records: one ffmpeg per connection, appending to the same playlist
	for range 3 {
		out, err := exec.Command("ffmpeg", "-v", "error", "-y", "-f", "lavfi", "-i", "testsrc=size=320x180:rate=30",
			"-t", strconv.Itoa(6), "-c:v", "libx264", "-preset", "ultrafast", "-g", "60",
			"-f", "hls", "-hls_time", "2", "-hls_playlist_type", "event", "-hls_flags", "append_list",
			"-hls_segment_filename", filepath.Join(dir, "%05d.ts"), playlist).CombinedOutput()
		if err != nil {
			t.Fatalf("recording: %v: %s", err, out)
		}
	}

	thumb, err := createVideoThumbnail(t.Context(), playlist)
	if err != nil {
		t.Fatalf("createVideoThumbnail: %v", err)
	}
	if !bytes.HasPrefix(thumb, []byte{0xff, 0xd8}) {
		t.Errorf("not a JPEG, starts with % x", thumb[:min(len(thumb), 4)])
	}
}
