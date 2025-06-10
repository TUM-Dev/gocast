package actions

import (
	"bufio"
	"context"
	"fmt"
	"github.com/tum-dev/gocast/runner/pkg/ptr"
	"io"
	"log/slog"
	"net/url"
	"os"
	"os/exec"
	"path"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"

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
		return AbortingError(fmt.Errorf("no stream end in context"))
	}
	recordingDir, ok := d["recordingDir"].(string)
	if !ok {
		return AbortingError(fmt.Errorf("no recordingDir in context"))
	}

	logger = logger.With("stream_id", streamID)

	metrics.ConvertingProgresses.With(metrics.With().Stream(streamID).L()).Inc()
	defer func() {
		metrics.ConvertingProgresses.With(metrics.With().Stream(streamID).L()).Dec()
	}()

	vodDir := path.Join(config.Config.StoragePath, fmt.Sprintf("%d", streamID), streamVersion)
	err := os.MkdirAll(vodDir, os.ModePerm)
	if err != nil {
		return AbortingError(fmt.Errorf("create VOD directory: %w", err))
	}

	recordingContent, err := os.ReadDir(recordingDir)
	if err != nil {
		return AbortingError(fmt.Errorf("read recordingDir: %w", err))
	}

	for _, entry := range recordingContent {
		if entry.IsDir() {
			log.Warn("found dir in recordingDir, skipping", "name", entry.Name())
			continue
		}
		if entry.Name() == "playlist.m3u8" {
			err = convertStream(ctx, logger, streamID, path.Join(recordingDir, entry.Name()), vodDir, entry.Name())
			continue
		}
	}

	vodUrl, err := url.JoinPath(config.Config.EdgeServer, fmt.Sprintf("%d", streamID), streamVersion, "playlist.m3u8")
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

func convertStream(ctx context.Context, logger *slog.Logger, streamID uint64, streamPath, vodDir string, playlistName string) error {
	input := "-i " + streamPath
	options := "-c copy -f hls -hls-time 240 -hls_playlist_type event -hls_flags append_list -hls_segment_filename " + path.Join(vodDir, "%05d.ts") + " " + path.Join(vodDir, playlistName)

	args := strings.Split(input, " ")
	args = append(args, strings.Split(options, " ")...)
	command := exec.CommandContext(ctx, "ffmpeg", args...)

	// give ffmpeg 10 seconds on sigterm (context cancellation) to shut down before sending sigkill.
	command.WaitDelay = 10 * time.Second

	log.Info("starting ffmpeg", "command", command.String())
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
		logger.Error("ffmpeg command failed", "error", err)
	} else {
		logger.Info("ffmpeg converting completed successfully", "stream_id", streamID)
	}
	return err
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
