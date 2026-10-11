package realtimehub

import (
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	protobuf "github.com/TUM-Dev/gocast/apiv2/protobuf/server"
)

// ReactionEvent says a viewer reacted to the live stream. Only the course's
// administrators see reactions: they are for the lecturer's display, not for the
// audience.
func ReactionEvent(streamID uint, reaction string) *protobuf.RealtimeEvent {
	ev := Event(streamID)
	ev.Audience = protobuf.RealtimeEvent_AUDIENCE_COURSE_ADMINS
	ev.Event = &protobuf.RealtimeEvent_StreamReaction{StreamReaction: &protobuf.StreamReactionEvent{Reaction: reaction}}
	return ev
}

// ReactionStatsEvent carries how the reactions of the last window split over the
// emoji, for the course's administrators.
func ReactionStatsEvent(streamID uint, shares []*protobuf.StreamReactionShare, window time.Duration) *protobuf.RealtimeEvent {
	ev := Event(streamID)
	ev.Audience = protobuf.RealtimeEvent_AUDIENCE_COURSE_ADMINS
	ev.Event = &protobuf.RealtimeEvent_StreamReactionStats{StreamReactionStats: &protobuf.StreamReactionStatsEvent{
		Shares:        shares,
		WindowSeconds: uint32(window / time.Second),
	}}
	return ev
}

// TimeChangedEvent says the lecture was rescheduled.
func TimeChangedEvent(streamID uint, start, end time.Time) *protobuf.RealtimeEvent {
	ev := Event(streamID)
	ev.Event = &protobuf.RealtimeEvent_TimeChanged{TimeChanged: &protobuf.StreamTimeChangedEvent{
		Start: timestamppb.New(start),
		End:   timestamppb.New(end),
	}}
	return ev
}
