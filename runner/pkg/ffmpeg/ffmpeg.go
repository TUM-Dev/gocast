package ffmpeg

import (
	"context"
	"fmt"
	"os/exec"

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

// Level returns the level of a FFProbeResult if in h264 or ""
func (r *FFProbeResult) Level() string {
	return gjson.Get(r.raw, "streams.0.level").String()
}

// Container returns the container(e.g. mp4, mkv, ...) of a FFProbeResult or ""
func (r *FFProbeResult) Container() string {
	return gjson.Get(r.raw, "format.format_name").String()
}

// StreamInfo holds information about a single stream
type StreamInfo struct {
	CodecType string
	CodecName string
	BitRate   int64
}

// Streams returns all streams with their codec information
func (r *FFProbeResult) Streams() []StreamInfo {
	nStreams := gjson.Get(r.raw, "streams.#").Int()
	streams := make([]StreamInfo, 0, nStreams)
	for i := 0; i < int(nStreams); i++ {
		codecType := gjson.Get(r.raw, fmt.Sprintf("streams.%d.codec_type", i)).String()
		codecName := gjson.Get(r.raw, fmt.Sprintf("streams.%d.codec_name", i)).String()
		bitRate := gjson.Get(r.raw, fmt.Sprintf("streams.%d.bit_rate", i)).Int()
		streams = append(streams, StreamInfo{
			CodecType: codecType,
			CodecName: codecName,
			BitRate:   bitRate,
		})
	}
	return streams
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
