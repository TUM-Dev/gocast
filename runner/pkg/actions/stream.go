package actions

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path"
	"strings"
	"time"

	"github.com/tum-dev/gocast/runner/pkg/ptr"

	"github.com/tum-dev/gocast/runner/config"
	"github.com/tum-dev/gocast/runner/pkg/metrics"
	"github.com/tum-dev/gocast/runner/protobuf"
)

func Stream(ctx context.Context, log *slog.Logger, notify chan *protobuf.Notification, d map[string]any, metrics *metrics.Broker) error {
	if ctx.Err() != nil {
		return AbortingError(ctx.Err())
	}
	streamID, ok := d["streamID"].(uint64)
	if !ok {
		return AbortingError(fmt.Errorf("no stream id in context"))
	}
	streamEnd, ok := d["streamEnd"].(time.Time)
	if !ok {
		return AbortingError(fmt.Errorf("no stream end in context"))
	}
	streamVersion, ok := d["streamVersion"].(string)
	if !ok {
		return AbortingError(fmt.Errorf("no stream end in context"))
	}
	globalOpts, ok := d["globalOpts"].(string)
	if !ok {
		globalOpts = ""
	}
	inputOpts, ok := d["inputOpts"].(string)
	if !ok {
		inputOpts = ""
	}
	outputOpts, ok := d["outputOpts"].(string)
	if !ok {
		outputOpts = "-c:a copy -c:v copy"
	}
	input, ok := d["input"].(string)
	if !ok {
		return AbortingError(fmt.Errorf("no input in context"))
	}

	metrics.Streams.With(metrics.With().Stream(streamID).Source(input).L()).Inc()
	defer func() {
		metrics.Streams.With(metrics.With().Stream(streamID).Source(input).L()).Dec()
	}()

	// e.g. ./storage/live/1/STREAM_VERSION_COMBINED
	liveRecDir := path.Join(config.Config.SegmentPath, fmt.Sprintf("%d", streamID), streamVersion)
	err := os.MkdirAll(liveRecDir, os.ModePerm)
	if err != nil {
		return AbortingError(err)
	}
	d["recordingDir"] = liveRecDir

	notify <- &protobuf.Notification{
		Data: &protobuf.Notification_StreamStart{
			StreamStart: &protobuf.StreamStartNotification{
				Stream:        &protobuf.StreamInfo{Id: ptr.Take(streamID)},
				StreamVersion: ptr.Take(protobuf.StreamVersion(protobuf.StreamVersion_value[streamVersion])),
				Url:           ptr.Take(fmt.Sprintf("%s/%s/%s/playlist.m3u8", config.Config.EdgeServer, config.Config.Hostname, path.Join(fmt.Sprintf("%d", streamID), streamVersion))),
			},
		},
	}

	args := []string{"-nostdin", "-y"} // noninteractive mode
	if globalOpts != "" {
		args = append(args, strings.Split(globalOpts, " ")...)
	}
	args = append(args, "-t", fmt.Sprintf("%.0f", time.Until(streamEnd).Seconds()))
	if inputOpts != "" {
		args = append(args, strings.Split(inputOpts, " ")...)
	}
	args = append(args, "-i", input)
	args = append(args, strings.Split(outputOpts, " ")...)
	args = append(args, strings.Split(`-f hls -hls_time 2 -hls_playlist_type event -hls_flags append_list -hls_segment_filename `+liveRecDir+"/%05d.ts "+liveRecDir+"/playlist.m3u8", " ")...)

	command := exec.CommandContext(ctx, "ffmpeg", args...)
	// give ffmpeg 10 seconds on sigterm (context cancellation) to shut down before sending sigkill.
	command.WaitDelay = 10 * time.Second

	log.Info("starting ffmpeg", "command", command.String())
	stderr, err := command.StderrPipe()
	if err != nil {
		return err
	}
	go logCmdPipe(log, stderr, []any{"stream", streamID, "logStream", "stderr"})

	stdo, err := command.StdoutPipe()
	if err != nil {
		return err
	}
	go logCmdPipe(log, stdo, []any{"stream", streamID, "logStream", "stdout"})
	err = command.Run()
	if err != nil {
		metrics.StreamErrors.With(metrics.With().Stream(streamID).Source(input).L()).Inc()
		return err
	}
	if streamEnd.After(time.Now()) {
		return fmt.Errorf("ffmpeg terminated before stream end")
	}
	return nil
}

func logCmdPipe(log *slog.Logger, pipe io.ReadCloser, fields []any) {
	scanner := bufio.NewScanner(pipe)
	for scanner.Scan() {
		log.Info("ffmpeg log", append([]any{"msg", scanner.Text()}, fields...)...)
	}
	log.Info("ffmpeg logstream ended", fields...)
}
