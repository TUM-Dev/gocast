package realtimehub

import (
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	protobuf "github.com/TUM-Dev/gocast/apiv2/protobuf/server"
)

// ReactionEvent says a viewer reacted to the live stream. Every viewer receives it:
// the emoji floats over the picture for the whole audience, and the lecturer's display
// highlights it. The event is the emoji and nothing else -- no user, no count -- so
// widening the audience reveals nobody. (The split of reactions over the emoji,
// ReactionStatsEvent, stays administrators-only.) Reusing this event instead of adding
// a second, viewer-facing one keeps one fan-out per reaction rather than two.
func ReactionEvent(streamID uint, reaction string) *protobuf.RealtimeEvent {
	ev := Event(streamID)
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
