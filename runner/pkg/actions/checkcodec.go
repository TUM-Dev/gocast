package actions

import (
	"context"
	"fmt"
	"log/slog"
	"path"

	"github.com/tum-dev/gocast/runner/pkg/ffmpeg"
	"github.com/tum-dev/gocast/runner/pkg/metrics"
	"github.com/tum-dev/gocast/runner/protobuf"
)

// CheckCodec probes the recording file and checks if it needs re-encoding.
// Sets "needsReencode" to true if the video is not h264 or exceeds 3Mbit/s bitrate,
// or if audio is not AAC.
func CheckCodec(ctx context.Context, logger *slog.Logger, _ chan *protobuf.Notification, d map[string]any, _ *metrics.Broker) error {
	recordingDir, ok := d["recordingDir"].(string)
	if !ok {
		return AbortingError(fmt.Errorf("no recordingDir in context"))
	}
	recording := path.Join(recordingDir, "playlist.m3u8")

	probe, err := ffmpeg.Probe(ctx, recording)
	if err != nil {
		return AbortingError(fmt.Errorf("ffprobe failed: %w", err))
	}

	needsReencode := false
	for _, stream := range probe.Streams() {
		if stream.CodecType == "video" {
			if stream.CodecName != "h264" {
				needsReencode = true
				logger.Info("video codec requires re-encoding", "codec", stream.CodecName)
				break
			}

			if stream.BitRate > 3000000 { // 3 Mbit/s in bits/s
				needsReencode = true
				logger.Info("video bitrate exceeds 3Mbit/s", "bitrate", stream.BitRate)
				break
			}
		}

		if stream.CodecType == "audio" {
			if stream.CodecName != "aac" {
				needsReencode = true
				logger.Info("audio codec requires re-encoding", "codec", stream.CodecName)
				break
			}
		}
	}

	d["needsReencode"] = needsReencode
	logger.Info("codec check completed", "needsReencode", needsReencode)

	return nil
}
