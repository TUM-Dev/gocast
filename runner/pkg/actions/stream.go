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

// selfStreamGrace is how long the RTMP source of a self-stream may stay gone after the
// scheduled end before the stream counts as over. Before the scheduled end an absent
// source is always waited for. The grace is what lets the lecturer lose their connection or
// take a break after the end without the recording being finalised under them.
const selfStreamGrace = 15 * time.Minute

// selfStreamLiveMin is how long an ffmpeg attempt has to run to count as having received a
// stream, as opposed to failing right away because nobody publishes.
const selfStreamLiveMin = 10 * time.Second

// selfStreamLastLive is the key in the job data for when a self-stream last delivered.
const selfStreamLastLive = "selfStreamLastLive"

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
	// The lecturer of a self-stream decides when it is over, not the schedule.
	selfStream, _ := d["selfStream"].(bool)
	if selfStream {
		lastLive, ok := d[selfStreamLastLive].(time.Time)
		if !ok {
			lastLive = time.Now()
			d[selfStreamLastLive] = lastLive
		}
		if streamEnd.Before(time.Now()) && time.Since(lastLive) > selfStreamGrace {
			return nil
		}
	} else if streamEnd.Before(time.Now()) {
		return nil
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
	if !selfStream {
		// Not for self-streams: ffmpeg ends them when the publisher disconnects, and a
		// duration would cut off a lecturer who runs over the scheduled end.
		args = append(args, "-t", fmt.Sprintf("%.0f", time.Until(streamEnd).Seconds()))
	}
	if inputOpts != "" {
		args = append(args, strings.Split(inputOpts, " ")...)
	}
	args = append(args, "-i", input)
	args = append(args, strings.Split(outputOpts, " ")...)
	args = append(args, strings.Split(`-f hls -hls_time 2 -hls_playlist_type event -hls_flags append_list -hls_segment_filename `+liveRecDir+"/%05d.ts "+liveRecDir+"/playlist.m3u8", " ")...)

	command := exec.CommandContext(ctx, "ffmpeg", args...)
	// Interrupt rather than kill, which is what CommandContext does by default: ffmpeg
	// writes the HLS trailer, #EXT-X-ENDLIST, only on a graceful shutdown, and MkVOD
	// reads a playlist without it as a live one and waits for segments forever.
	command.Cancel = func() error { return command.Process.Signal(os.Interrupt) }
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
	started := time.Now()
	err = command.Run()
	if selfStream {
		if time.Since(started) >= selfStreamLiveMin {
			d[selfStreamLastLive] = time.Now()
		}
		if err != nil {
			metrics.StreamErrors.With(metrics.With().Stream(streamID).Source(input).L()).Inc()
			return err
		}
		// A clean exit is the publisher disconnecting. It may come back (connection
		// loss, a break), so retry: whether it is over is decided at the top, by
		// the scheduled end and the grace.
		return fmt.Errorf("self-stream publisher disconnected")
	}
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
