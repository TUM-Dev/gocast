package actions

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"path"

	"github.com/tum-dev/gocast/runner/pkg/ffmpeg"
	"github.com/tum-dev/gocast/runner/pkg/metrics"
	"github.com/tum-dev/gocast/runner/protobuf"
)

// CheckVoD probes both the livestream and the VoD and checks if they are identical.
func CheckVoD(ctx context.Context, logger *slog.Logger, _ chan *protobuf.Notification, d map[string]any, metrics *metrics.Broker) error {
	const DurationToleranceSeconds = 10.0

	streamID, ok := d["streamID"].(uint64)
	if !ok {
		return AbortingError(fmt.Errorf("no stream id in context"))
	}
	streamVersion, ok := d["streamVersion"].(string)
	if !ok {
		return AbortingError(fmt.Errorf("no stream version in context"))
	}
	recording, ok := d["recordingDir"].(string)
	if !ok {
		return AbortingError(fmt.Errorf("no recordingDir in context"))
	}
	recording = path.Join(recording, "playlist.m3u8")
	vod, ok := d["vodPath"].(string)
	if !ok {
		return AbortingError(fmt.Errorf("no vodPath in context"))
	}

	isHealthy := false
	defer func() {
		d["vodHealthy"] = isHealthy
		if !isHealthy {
			metrics.ConvertingErrors.With(metrics.With().Stream(streamID).StreamVersion(streamVersion).L()).Inc()
		}
	}()

	probe, err := ffmpeg.Probe(ctx, recording)
	if err != nil {
		return AbortingError(fmt.Errorf("probe livestream: %w", err))
	}
	liveDuration := probe.Duration()
	probe, err = ffmpeg.Probe(ctx, vod)
	if err != nil {
		return AbortingError(fmt.Errorf("probe vod: %w", err))
	}
	vodDuration := probe.Duration()
	if math.Abs(liveDuration-vodDuration) > DurationToleranceSeconds {
		return AbortingError(fmt.Errorf("livestream duration (%f) and recording duration (%f) diverge by more than 10 s", liveDuration, vodDuration))
	}
	isHealthy = true
	logger.With("stream", streamID, "stream_version", streamVersion).Info("CheckVoD successful")
	return nil
}
