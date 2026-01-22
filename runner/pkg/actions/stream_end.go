package actions

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/tum-dev/gocast/runner/pkg/metrics"
	"github.com/tum-dev/gocast/runner/pkg/ptr"
	"github.com/tum-dev/gocast/runner/protobuf"
)

// StreamEnd is an action who's sole purpose is to notify gocast about the end of a stream.
// the only reason it is a separate action is to avoid sending unnecessary
// stream_end notifications if Stream errors.
func StreamEnd(_ context.Context, _ *slog.Logger, notify chan *protobuf.Notification, d map[string]any, metrics *metrics.Broker) error {
	streamID, ok := d["streamID"].(uint64)
	if !ok {
		return AbortingError(fmt.Errorf("no stream id in context"))
	}
	notify <- &protobuf.Notification{
		Data: &protobuf.Notification_StreamEnd{
			StreamEnd: &protobuf.StreamEndNotification{
				Stream: &protobuf.StreamInfo{Id: ptr.Take(streamID)},
			},
		},
	}
	return nil
}
