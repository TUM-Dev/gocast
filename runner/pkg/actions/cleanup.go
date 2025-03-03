package actions

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"os"
	"os/exec"
	"path"

	"github.com/tum-dev/gocast/runner/config"
	"github.com/tum-dev/gocast/runner/pkg/metrics"
	"github.com/tum-dev/gocast/runner/pkg/probe"
	"github.com/tum-dev/gocast/runner/protobuf"
)

// Cleanup is an action that removes all contents in recordingDir if vodHealthy is true.
// otherwise, it uses the mv command, to move the broken live recording to {mass_dir}/broken.
func Cleanup(ctx context.Context, log *slog.Logger, _ chan *protobuf.Notification, d map[string]any, metrics *metrics.Broker) error {
	recDir, ok := d["recordingDir"].(string)
	if !ok {
		return AbortingError(fmt.Errorf("no recordingDir in context"))
	}
	vodHealthy, ok := d["vodHealthy"].(bool)
	if !ok {
		return AbortingError(fmt.Errorf("no recordingDir in context"))
	}
	if vodHealthy {
		err := os.RemoveAll(recDir)
		if err != nil {
			return AbortingError(fmt.Errorf("remove vod dir: %w", err))
		}
		return nil
	}
	// vod unhealthy, move to mass:
	dst := path.Join(recDir, config.Config.StoragePath, "broken")
	err := os.MkdirAll(dst, os.ModePerm)
	if err != nil && !os.IsExist(err) {
		return AbortingError(fmt.Errorf("create broken vod directory: %w", err))
	}
	log.Info("moving broken vod", "scr", recDir, "dst", dst)
	cmd := exec.CommandContext(ctx, "mv", dst)
	err = cmd.Start()
	if err != nil {
		return AbortingError(fmt.Errorf("start move broken recording command: %w", err))
	}
	err = cmd.Wait()
	if err != nil {
		return AbortingError(fmt.Errorf("wait move broken recording command: %w", err))
	}
	return nil
}

// ProbeVodHealth checks the duration of the live and vod playlists and sets d["vodHealthy"] to false if it detects
// a difference of the stream lengths of more than 10 seconds or if any of the probes fail or return no streams.
func ProbeVodHealth(ctx context.Context, log *slog.Logger, _ chan *protobuf.Notification, d map[string]any, metrics *metrics.Broker) error {
	recDir, ok := d["recordingDir"].(string)
	if !ok {
		return AbortingError(fmt.Errorf("no recordingDir in context"))
	}
	vodDir, ok := d["vodDir"].(string)
	if !ok {
		return AbortingError(fmt.Errorf("no vodDir in context"))
	}

	probeLive, err := probe.Probe(ctx, path.Join(recDir, "playlist.m3u8"))
	if err != nil {
		log.Error("probe livestream", "err", err)
		d["vodHealthy"] = false
		return nil
	}
	probeVod, err := probe.Probe(ctx, path.Join(vodDir, "playlist.m3u8"))
	if err != nil {
		log.Error("probe vod", "err", err)
		d["vodHealthy"] = false
		return nil
	}

	if len(probeLive.Streams) == 0 {
		log.Error("live streams # == 0")
		d["vodHealthy"] = false
		return nil
	}

	if len(probeVod.Streams) == 0 {
		log.Error("vod streams # == 0")
		d["vodHealthy"] = false
		return nil
	}

	durationLive, err := probeLive.Streams[0].DurationFloat()
	if err != nil {
		log.Error("parse live stream duration", "err", err)
		d["vodHealthy"] = false
		return nil
	}
	durationVod, err := probeVod.Streams[0].DurationFloat()
	if err != nil {
		log.Error("parse vod stream duration", "err", err)
		d["vodHealthy"] = false
		return nil
	}
	if math.Max(durationVod, durationLive)-math.Min(durationVod, durationLive) > 10 {
		log.Error("vod and live durations of by > 10s", "vodDuration", durationVod, "liveDuration", durationLive)
		d["vodHealthy"] = false
		return nil
	}
	log.Info("vod is healthy")
	d["vodHealthy"] = true
	return nil
}
