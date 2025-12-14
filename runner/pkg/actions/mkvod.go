package actions

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/url"
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

// MkVOD takes a stream that was streamed and moves the hls stream to long term storage.
// Additionally, the playlist type is transformed from event to VOD.
func MkVOD(ctx context.Context, logger *slog.Logger, notify chan *protobuf.Notification, d map[string]any, metrics *metrics.Broker) error {
	streamID, ok := d["streamID"].(uint64)
	if !ok {
		return AbortingError(fmt.Errorf("no stream id in context"))
	}
	streamVersion, ok := d["streamVersion"].(string)
	if !ok {
		return AbortingError(fmt.Errorf("no stream version in context"))
	}
	var recording string
	if rec, ok := d["recording"]; ok {
		recording = rec.(string)
	} else if dir, ok := d["recordingDir"]; ok {
		recording = path.Join(dir.(string), "playlist.m3u8")
	} else {
		return AbortingError(fmt.Errorf("no recording or recordingDir in context"))
	}

	metrics.ConvertingProgresses.With(metrics.With().Stream(streamID).L()).Inc()
	defer metrics.ConvertingProgresses.With(metrics.With().Stream(streamID).L()).Dec()

	vodDir := path.Join(config.Config.StoragePath, fmt.Sprintf("%d", streamID), streamVersion)
	err := os.MkdirAll(vodDir, os.ModePerm)
	if err != nil {
		return AbortingError(fmt.Errorf("create VOD directory: %w", err))
	}

	// Check if re-encoding is needed
	var reencode bool
	if needsReencode, ok := d["needsReencode"]; ok {
		reencode = needsReencode.(bool)
	}

	var videoCodec, audioCodec string
	if reencode {
		logger.Info("re-encoding required, transcoding video")
		videoCodec = "libx264"
		audioCodec = "aac"
	} else {
		logger.Info("no re-encoding needed, using copy codec")
		videoCodec = "copy"
		audioCodec = "copy"
	}

	err = convertStream(ctx, logger, streamID, recording, vodDir, "playlist.m3u8", videoCodec, audioCodec)
	if err != nil {
		return AbortingError(fmt.Errorf("convert stream: %w", err))
	}
	d["vodPath"] = path.Join(vodDir, "playlist.m3u8")

	vodUrl, err := url.JoinPath(config.Config.EdgeServer, "vod", fmt.Sprintf("%d", streamID), streamVersion, "playlist.m3u8")
	if err != nil {
		return fmt.Errorf("join vod url: %w", err)
	}
	notify <- &protobuf.Notification{
		Data: &protobuf.Notification_VodReady{
			VodReady: &protobuf.VODReadyNotification{
				Stream:        &protobuf.StreamInfo{Id: ptr.Take(streamID)},
				StreamVersion: ptr.Take(protobuf.StreamVersion(protobuf.StreamVersion_value[streamVersion])),
				Url:           ptr.Take(vodUrl),
			},
		},
	}
	return nil
}

func convertStream(ctx context.Context, logger *slog.Logger, streamID uint64, streamPath, vodDir string, playlistName string, videoCodec string, audioCodec string) error {
	input := "-i " + streamPath

	// Build codec options based on parameters
	codecOpts := fmt.Sprintf("-c:v %s -c:a %s", videoCodec, audioCodec)

	// Add bitrate limit if re-encoding video
	if videoCodec != "copy" {
		codecOpts += " -b:v 3M"
	}

	options := codecOpts + " -f hls -hls_time 20 -hls_playlist_type vod -hls_flags append_list -hls_segment_filename " + path.Join(vodDir, "%05d.ts") + " " + path.Join(vodDir, playlistName)

	args := strings.Split(input, " ")
	args = append(args, strings.Split(options, " ")...)
	command := exec.CommandContext(ctx, "ffmpeg", args...)

	// give ffmpeg 10 seconds on sigterm (context cancellation) to shut down before sending sigkill.
	command.WaitDelay = 10 * time.Second

	logger.Info("starting ffmpeg", "command", command.String())
	stderr, err := command.StderrPipe()
	if err != nil {
		return err
	}
	go logCmdPipe(logger, stderr, []any{"stream", streamID, "logStream", "stderr"})

	stdo, err := command.StdoutPipe()
	if err != nil {
		return err
	}
	go logCmdPipe(logger, stdo, []any{"stream", streamID, "logStream", "stdout"})
	err = command.Run()
	if err != nil {
		return fmt.Errorf("ffmpeg failed: %w", err)
	}
	return nil
}

// vodFromEventPlst modifies an HLS playlist from EVENT to VOD
func vodFromEventPlst(playlist io.Reader) string {
	var lines []string
	hasEndlist := false

	scanner := bufio.NewScanner(playlist)
	for scanner.Scan() {
		line := scanner.Text()

		// Replace #EXT-X-PLAYLIST-TYPE:EVENT with #EXT-X-PLAYLIST-TYPE:VOD
		if strings.Contains(line, "#EXT-X-PLAYLIST-TYPE:EVENT") {
			line = "#EXT-X-PLAYLIST-TYPE:VOD"
		}

		// Check if #EXT-X-ENDLIST is already present
		if line == "#EXT-X-ENDLIST" {
			hasEndlist = true
		}

		lines = append(lines, line)
	}

	// Append #EXT-X-ENDLIST if missing
	if !hasEndlist {
		lines = append(lines, "#EXT-X-ENDLIST")
	}

	return strings.Join(lines, "\n")
}
