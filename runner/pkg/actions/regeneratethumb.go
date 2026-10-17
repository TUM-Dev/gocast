package actions

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/tum-dev/gocast/runner/pkg/metrics"
	"github.com/tum-dev/gocast/runner/pkg/ptr"
	"github.com/tum-dev/gocast/runner/protobuf"
)

// RegenerateThumb generates a thumbnail from an already-recorded VOD and sends it
// to the control plane in a notification. Unlike MkThumb, which reads from a local
// recording directory produced by a just-finished live stream, this reads directly
// from the VOD's playlist URL.
func RegenerateThumb(_ context.Context, logger *slog.Logger, notify chan *protobuf.Notification, d map[string]any, _ *metrics.Broker) error {
	logger.Info("regenerating thumbnail")
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
	playlistURL, ok := d["playlistURL"].(string)
	if !ok || playlistURL == "" {
		return AbortingError(fmt.Errorf("no playlist url in context"))
	}

	thumb, err := createVideoThumbnail(playlistURL)
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
