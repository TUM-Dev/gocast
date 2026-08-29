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
	streamEnd := &protobuf.StreamEndNotification{
		Stream: &protobuf.StreamInfo{Id: ptr.Take(streamID)},
	}
	if versionStr, ok := d["streamVersion"].(string); ok {
		if v, found := protobuf.StreamVersion_value[versionStr]; found {
			sv := protobuf.StreamVersion(v)
			streamEnd.StreamVersion = &sv
		}
	}
	notify <- &protobuf.Notification{
		Data: &protobuf.Notification_StreamEnd{
			StreamEnd: streamEnd,
		},
	}
	return nil
}
