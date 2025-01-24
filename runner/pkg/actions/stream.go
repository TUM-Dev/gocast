package actions

import (
	"bufio"
	"context"
	"fmt"
	"github.com/tum-dev/gocast/runner/config"
	"github.com/tum-dev/gocast/runner/pkg/ptr"
	"github.com/tum-dev/gocast/runner/protobuf"
	"log/slog"
	"os"
	"os/exec"
	"path"
	"strings"
	"time"
)

func Stream(ctx context.Context, log *slog.Logger, notify chan *protobuf.Notification, d map[string]any) error {
	streamID, ok := d["streamID"].(*uint64)
	if !ok {
		return AbortingError(fmt.Errorf("action/strean: no stream id in context"))
	}
	streamEnd, ok := d["streamEnd"].(time.Time)
	if !ok {
		return AbortingError(fmt.Errorf("action/strean: no stream end in context"))
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
		return AbortingError(fmt.Errorf("action/strean: no input in context"))
	}

	log.Info("streaming")
	liveRecDir := path.Join(config.Config.SegmentPath, fmt.Sprintf("%d", *streamID))
	err := os.Mkdir(liveRecDir, os.ModePerm)
	if err != nil {
		return AbortingError(err)
	}
	d["recording"] = liveRecDir

	notify <- &protobuf.Notification{
		Data: &protobuf.Notification_StreamStart{
			StreamStart: &protobuf.StreamStartNotification{
				Stream: &protobuf.StreamInfo{Id: streamID},
				Url:    ptr.Take(fmt.Sprintf("%s/%s/%s/%d.m3u8", config.Config.EdgeServer, config.Config.Hostname, liveRecDir, *streamID)),
			},
		},
	}
	args := strings.Split(globalOpts, " ")
	args = append(args, "-t", fmt.Sprintf("%.0f", time.Until(streamEnd).Seconds()))
	args = append(args, strings.Split(inputOpts, " ")...)
	args = append(args, "-i", input)
	args = append(args, strings.Split(outputOpts, " ")...)
	args = append(args, strings.Split(`-f hls -hls_time 2 -hls_playlist_type event -hls_flags append_list -hls_segment_filename `+liveRecDir+"/%05d.ts "+liveRecDir+"/playlist.m3u8", " ")...)
	command := exec.CommandContext(ctx, "ffmpeg", args...)
	log.Info("starting ffmpeg", "command", command.String())
	stderr, err := command.StderrPipe()
	if err != nil {
		return AbortingError(err)
	}
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			log.Debug(scanner.Text())
		}
	}()
	err = command.Run()
	if err != nil {
		return err
	}
	return nil
}
