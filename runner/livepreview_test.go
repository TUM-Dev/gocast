package runner

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"path/filepath"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/tum-dev/gocast/runner/pkg/ptr"
	"github.com/tum-dev/gocast/runner/protobuf"
)

func TestGenerateLivePreviewRejectsBadURLs(t *testing.T) {
	r := newTestRunner()
	for name, url := range map[string]string{
		"empty":      "",
		"local file": "/etc/passwd",
		"file url":   "file:///etc/passwd",
		"concat":     "concat:/etc/passwd|x",
	} {
		t.Run(name, func(t *testing.T) {
			resp, err := r.GenerateLivePreview(context.Background(), &protobuf.LivePreviewRequest{PlaylistUrl: ptr.Take(url)})
			if resp != nil {
				t.Errorf("got a response: %v", resp)
			}
			if got := status.Code(err); got != codes.InvalidArgument {
				t.Errorf("status code: got %v, want %v", got, codes.InvalidArgument)
			}
		})
	}
}

func TestGenerateLivePreviewReturnsJPEG(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not installed")
	}
	dir := t.TempDir()
	playlist := filepath.Join(dir, "playlist.m3u8")
	out, err := exec.Command("ffmpeg", "-f", "lavfi", "-i", "testsrc=duration=6:size=320x240:rate=10",
		"-c:v", "libx264", "-pix_fmt", "yuv420p", "-f", "hls", "-hls_time", "2", "-hls_list_size", "0", playlist).CombinedOutput()
	if err != nil {
		t.Skipf("can't make a test stream: %v\n%s", err, out)
	}
	srv := httptest.NewServer(http.FileServer(http.Dir(dir)))
	defer srv.Close()

	resp, err := newTestRunner().GenerateLivePreview(context.Background(),
		&protobuf.LivePreviewRequest{PlaylistUrl: ptr.Take(srv.URL + "/playlist.m3u8")})
	if err != nil {
		t.Fatalf("GenerateLivePreview: %v", err)
	}
	if b := resp.GetThumbnail(); len(b) < 3 || b[0] != 0xff || b[1] != 0xd8 {
		t.Errorf("not a JPEG: %d bytes", len(b))
	}
}
