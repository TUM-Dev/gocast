package actions

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"

	"github.com/joschahenningsen/thumbgen"

	"github.com/tum-dev/gocast/runner/pkg/metrics"
	"github.com/tum-dev/gocast/runner/pkg/ptr"
	"github.com/tum-dev/gocast/runner/protobuf"
)

// MkThumb Generates a thumbnail from a stream and sends it to the control plane in a notification
func MkThumb(_ context.Context, logger *slog.Logger, notify chan *protobuf.Notification, d map[string]any, metrics *metrics.Broker) error {
	logger.Info("generating thumbnail")
	streamID, ok := d["streamID"].(uint64)
	if !ok {
		return AbortingError(fmt.Errorf("no stream id in context"))
	}
	var streamVersion *protobuf.StreamVersion
	if sv, ok := d["streamVersion"].(string); ok {
		streamVersion = ptr.Take(protobuf.StreamVersion(protobuf.StreamVersion_value[sv]))
	} else {
		return AbortingError(fmt.Errorf("no stream version in context"))
	}
	recordingDir, ok := d["recordingDir"].(string)
	if !ok {
		return AbortingError(fmt.Errorf("no recordingDir in context"))
	}

	thumb, err := createVideoThumbnail(recordingDir + "/playlist.m3u8")
	if err != nil {
		return AbortingError(fmt.Errorf("error creating thumbnail: %v", err))
	}

	notify <- &protobuf.Notification{
		Data: &protobuf.Notification_ThumbnailReady{ThumbnailReady: &protobuf.ThumbnailReadyNotification{
			Stream:        &protobuf.StreamInfo{Id: ptr.Take(streamID)},
			StreamVersion: streamVersion,
			Thumbnail:     thumb,
		}},
	}
	return nil
}

// createVideoThumbnail creates a thumbnail from the given video file

const (
	thumbnailWidth         = 720 // Width of the generated thumbnail in pixels
	jpegCompressionQuality = 90  // JPEG compression quality (0-100)
)

func createVideoThumbnail(source string) ([]byte, error) {
	g, err := thumbgen.New(source, thumbnailWidth, 1, "", thumbgen.WithJpegCompression(jpegCompressionQuality))
	if err != nil {
		return nil, err
	}
	path, err := g.GenerateOne()
	if err != nil {
		return nil, err
	}
	thumb, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = thumb.Close()
		_ = os.Remove(path)
	}()
	return io.ReadAll(thumb)
}
