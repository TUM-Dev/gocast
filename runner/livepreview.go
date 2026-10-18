package runner

import (
	"context"
	"os/exec"
	"strings"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/tum-dev/gocast/runner/protobuf"
)

// livePreviewTimeout bounds the ffmpeg run. The server refreshes previews every minute
// and gives up on its side after 15s, so a run that outlives this has no one waiting.
const livePreviewTimeout = 15 * time.Second

// GenerateLivePreview grabs a frame from the end of a running livestream. It is
// deliberately not a job: nothing is tracked or notified, the caller waits for the JPEG.
func (r *Runner) GenerateLivePreview(ctx context.Context, req *protobuf.LivePreviewRequest) (*protobuf.LivePreviewResponse, error) {
	url := req.GetPlaylistUrl()
	if url == "" {
		return nil, status.Errorf(codes.InvalidArgument, "no playlist url given")
	}
	// ffmpeg opens whatever it is given, local files included. Previews only ever
	// come from the (signed) playlist over HTTP.
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		return nil, status.Errorf(codes.InvalidArgument, "playlist url must be http(s)")
	}

	ctx, cancel := context.WithTimeout(ctx, livePreviewTimeout)
	defer cancel()
	// Same invocation the worker used: the last seconds of the stream, one frame.
	// The error is deliberately not logged with the URL, it carries a signed token.
	thumb, err := exec.CommandContext(ctx, "ffmpeg",
		"-protocol_whitelist", "http,https,tcp,tls,crypto",
		"-sseof", "-3",
		"-i", url,
		"-vframes", "1",
		"-update", "1",
		"-vf", "scale=720:-1",
		"-q:v", "1",
		"-c:v", "mjpeg",
		"-f", "mjpeg",
		"pipe:1").Output()
	if err != nil {
		r.log.Warn("live preview failed", "err", err)
		return nil, status.Errorf(codes.Internal, "ffmpeg: %v", err)
	}
	if len(thumb) == 0 {
		return nil, status.Errorf(codes.Internal, "ffmpeg produced no frame")
	}
	return &protobuf.LivePreviewResponse{Thumbnail: thumb}, nil
}
