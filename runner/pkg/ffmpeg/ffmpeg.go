package ffmpeg

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/tidwall/gjson"
)

// Duration returns the duration of the first stream in an FFProbeResult or 0
func (r *FFProbeResult) Duration() float64 {
	return gjson.Get(r.raw, "format.duration").Float()
}

// Codec returns the codec (e.g. h264) of an audio or video stream in an FFProbeResult or ""
func (r *FFProbeResult) Codec(codecType string) string {
	nStreams := gjson.Get(r.raw, "streams.#").Int()
	for i := 0; i < int(nStreams); i++ {
		if gjson.Get(r.raw, fmt.Sprintf("streams.%d.codec_type", i)).String() == codecType {
			return gjson.Get(r.raw, fmt.Sprintf("streams.%d.codec_name", i)).String()
		}
	}
	return ""
}

// PixelFormat returns the pixel format (e.g. yuv420p) of the first video stream in an
// FFProbeResult or ""
func (r *FFProbeResult) PixelFormat() string {
	nStreams := gjson.Get(r.raw, "streams.#").Int()
	for i := 0; i < int(nStreams); i++ {
		if gjson.Get(r.raw, fmt.Sprintf("streams.%d.codec_type", i)).String() == "video" {
			return gjson.Get(r.raw, fmt.Sprintf("streams.%d.pix_fmt", i)).String()
		}
	}
	return ""
}

// Dimensions returns the width and height of the first video stream in an
// FFProbeResult, or zeros
func (r *FFProbeResult) Dimensions() (width, height int) {
	nStreams := gjson.Get(r.raw, "streams.#").Int()
	for i := 0; i < int(nStreams); i++ {
		if gjson.Get(r.raw, fmt.Sprintf("streams.%d.codec_type", i)).String() == "video" {
			return int(gjson.Get(r.raw, fmt.Sprintf("streams.%d.width", i)).Int()),
				int(gjson.Get(r.raw, fmt.Sprintf("streams.%d.height", i)).Int())
		}
	}
	return 0, 0
}

// Level returns the level of a FFProbeResult if in h264 or ""
func (r *FFProbeResult) Level() string {
	return gjson.Get(r.raw, "streams.0.level").String()
}

// Container returns the container(e.g. mp4, mkv, ...) of a FFProbeResult or ""
func (r *FFProbeResult) Container() string {
	return gjson.Get(r.raw, "format.format_name").String()
}

// FFProbeResult holds the results of a ffprobe execution
type FFProbeResult struct {
	raw string
}

// Probe ffprobes the given file
func Probe(ctx context.Context, file string) (*FFProbeResult, error) {
	out, err := exec.CommandContext(ctx, "ffprobe",
		"-v", "quiet",
		"-print_format", "json",
		"-show_format", "-show_streams", file).CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("execute ffprobe: %w", err)
	}
	return &FFProbeResult{raw: string(out)}, err
}

// MeasureDuration returns how long the first video stream of a file runs, in seconds,
// from the timestamps of its packets. For files whose header has no duration, as a
// browser's MediaRecorder writes them; it reads the whole file, but decodes nothing.
func MeasureDuration(ctx context.Context, file string) (float64, error) {
	out, err := exec.CommandContext(ctx, "ffprobe",
		"-v", "error",
		"-select_streams", "v:0",
		"-show_entries", "packet=pts_time,duration_time",
		"-of", "csv=p=0", file).Output()
	if err != nil {
		return 0, fmt.Errorf("ffprobe packets: %w", err)
	}
	var end float64
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		fields := strings.Split(line, ",")
		pts, err := strconv.ParseFloat(fields[0], 64)
		if err != nil {
			continue
		}
		var dur float64
		if len(fields) > 1 {
			dur, _ = strconv.ParseFloat(fields[1], 64)
		}
		end = max(end, pts+dur)
	}
	return end, nil
}
