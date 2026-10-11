// Package realtimehub fans per-stream events out to the sockets watching a stream.
//
// It lives outside api/ and apiv2/ on purpose: the v1 producers that still publish
// here are deleted with api/, and the socket that consumes it is only one client.
// Producers and consumers see the Hub interface, never the implementation, so the
// in-process hub can be swapped for a message bus once there is more than one server
// process without touching either side.
package realtimehub

import (
	"context"

	"google.golang.org/protobuf/types/known/timestamppb"

	protobuf "github.com/TUM-Dev/gocast/apiv2/protobuf/server"
)

// Hub routes RealtimeEvents to the subscribers of one stream.
type Hub interface {
	// Publish delivers the event to every current subscriber of the stream. It never
	// blocks on a subscriber; see Memory for what happens to a slow one.
	Publish(ctx context.Context, streamID uint, event *protobuf.RealtimeEvent)

	// Subscribe registers a subscriber. The channel is closed after cancel is called,
	// or when the hub drops the subscriber for falling behind, in which case the last
	// event it receives is a ResyncEvent. cancel is safe to call more than once.
	Subscribe(streamID uint) (events <-chan *protobuf.RealtimeEvent, cancel func())

	// Viewers is the number of subscribers the stream has right now.
	Viewers(streamID uint) int
}

// Event fills in the envelope of a RealtimeEvent for everyone watching the stream.
// The oneof is the caller's to set.
func Event(streamID uint) *protobuf.RealtimeEvent {
	return &protobuf.RealtimeEvent{
		StreamId: uint32(streamID),
		At:       timestamppb.Now(),
		Audience: protobuf.RealtimeEvent_AUDIENCE_ALL,
	}
}

// LiveEvent says the stream went live or ended.
func LiveEvent(streamID uint, live bool) *protobuf.RealtimeEvent {
	ev := Event(streamID)
	ev.Event = &protobuf.RealtimeEvent_Live{Live: &protobuf.StreamLiveEvent{Live: live}}
	return ev
}

// TitleEvent says the stream was renamed.
func TitleEvent(streamID uint, title string) *protobuf.RealtimeEvent {
	ev := Event(streamID)
	ev.Event = &protobuf.RealtimeEvent_Title{Title: &protobuf.StreamTitleEvent{Title: title}}
	return ev
}

// DescriptionEvent carries the stream's new description, already rendered to HTML.
func DescriptionEvent(streamID uint, html string) *protobuf.RealtimeEvent {
	ev := Event(streamID)
	ev.Event = &protobuf.RealtimeEvent_Description{Description: &protobuf.StreamDescriptionEvent{Html: html}}
	return ev
}

// ViewersEvent carries the stream's viewer count.
func ViewersEvent(streamID uint, viewers int) *protobuf.RealtimeEvent {
	ev := Event(streamID)
	ev.Event = &protobuf.RealtimeEvent_Viewers{Viewers: &protobuf.ViewersEvent{Viewers: uint32(viewers)}}
	return ev
}

// ResyncEvent tells a client its view is stale and must be fetched again.
func ResyncEvent(streamID uint, reason string) *protobuf.RealtimeEvent {
	ev := Event(streamID)
	ev.Event = &protobuf.RealtimeEvent_Resync{Resync: &protobuf.ResyncEvent{Reason: reason}}
	return ev
}
