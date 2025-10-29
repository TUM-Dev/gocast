package actions

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"os"
	"path"
	"time"

	"github.com/tum-dev/gocast/runner/pkg/ffmpeg"
	"github.com/tum-dev/gocast/runner/pkg/metrics"
	"github.com/tum-dev/gocast/runner/protobuf"
)

// CheckVoD probes both the livestream and the VoD and checks if they are identical.
func CheckVoD(ctx context.Context, logger *slog.Logger, _ chan *protobuf.Notification, d map[string]any, metrics *metrics.Broker) error {
	duration, err := checkVoD(ctx, logger, d, metrics)
	recordingDir, ok := d["recordingDir"].(string)
	if !ok {
		return AbortingError(fmt.Errorf("missing recordingDir"))
	}
	formatString := ".del-%d"
	if err != nil {
		// Add a keep flag to the folder, add duration of 2h on average to the keep
		formatString = ".keep-%d"
		duration = 2 * time.Hour
	}
	filename := fmt.Sprintf(formatString, time.Now().Add(duration).Unix())
	err2 := writeManagementFile(recordingDir, filename)
	return errors.Join(err, err2)
}

func writeManagementFile(recordingDir string, filename string) error {
	file, err := os.Create(path.Join(recordingDir, filename))
	if err != nil {
		return AbortingError(err)
	}
	err = file.Close()
	if err != nil {
		return AbortingError(err)
	}
	return nil
}

func checkVoD(ctx context.Context, logger *slog.Logger, d map[string]any, metrics *metrics.Broker) (time.Duration, error) {
	const DurationToleranceSeconds = 10.0

	streamID, ok := d["streamID"].(uint64)
	if !ok {
		return time.Duration(0), AbortingError(fmt.Errorf("no stream id in context"))
	}
	streamVersion, ok := d["streamVersion"].(string)
	if !ok {
		return time.Duration(0), AbortingError(fmt.Errorf("no stream version in context"))
	}
	recordingDir, ok := d["recordingDir"].(string)
	if !ok {
		return time.Duration(0), AbortingError(fmt.Errorf("no recordingDir in context"))
	}
	recording := path.Join(recordingDir, "playlist.m3u8")
	vod, ok := d["vodPath"].(string)
	if !ok {
		return time.Duration(0), AbortingError(fmt.Errorf("no vodPath in context"))
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
		return time.Duration(0), AbortingError(fmt.Errorf("probe livestream: %w", err))
	}
	liveDuration := probe.Duration()
	probe, err = ffmpeg.Probe(ctx, vod)
	if err != nil {
		return time.Duration(0), AbortingError(fmt.Errorf("probe vod: %w", err))
	}
	vodDuration := probe.Duration()
	if math.Abs(liveDuration-vodDuration) > DurationToleranceSeconds {
		return time.Duration(0), AbortingError(fmt.Errorf("livestream duration (%f) and recording duration (%f) diverge by more than 10 s", liveDuration, vodDuration))
	}
	isHealthy = true
	logger.With("stream", streamID, "stream_version", streamVersion).Info("CheckVoD successful")

	duration, err := time.ParseDuration(fmt.Sprintf("%.0fs", liveDuration))
	if err != nil {
		return time.Duration(0), AbortingError(err)
	}

	return duration, nil
}
