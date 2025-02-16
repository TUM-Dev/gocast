package actions

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"os"
	"path"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/tum-dev/gocast/runner/config"
	"github.com/tum-dev/gocast/runner/pkg/metrics"
	"github.com/tum-dev/gocast/runner/pkg/ptr"
	"github.com/tum-dev/gocast/runner/protobuf"
)

// MkVOD takes a stream that was streamed and moves the hls stream to long term storage.
// Additionally, the playlist type is transformed from event to VOD.
func MkVOD(_ context.Context, _ *slog.Logger, notify chan *protobuf.Notification, d map[string]any, metrics *metrics.Broker) error {
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
			srcPlst, err := os.Open(path.Join(recordingDir, entry.Name()))
			if err != nil {
				return AbortingError(fmt.Errorf("open recording playlist: %w", err))
			}
			dstPlstS := vodFromEventPlst(srcPlst)
			dstPlst, err := os.Create(path.Join(vodDir, entry.Name()))
			if err != nil {
				return AbortingError(fmt.Errorf("create vod playlist: %w", err))
			}
			_, err = io.WriteString(dstPlst, dstPlstS)
			if err != nil {
				return fmt.Errorf("write vod to playlist: %w", err)
			}
			_ = dstPlst.Close()
			continue
		}
		err = copyFile(path.Join(recordingDir, entry.Name()), path.Join(vodDir, entry.Name()))
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

func copyFile(sourcePath, destPath string) error {
	inputFile, err := os.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("open source file: %w", err)
	}
	defer inputFile.Close()

	outputFile, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("open dest file: %w", err)
	}
	defer outputFile.Close()

	_, err = io.Copy(outputFile, inputFile)
	if err != nil {
		return fmt.Errorf("copy to dest from source: %w", err)
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
