package realtimehub

import (
	protobuf "github.com/TUM-Dev/gocast/apiv2/protobuf/server"
)

// MessageEvent carries a new chat message. A message that awaits approval goes to the
// course's administrators and its author alone, which is what authorID is for.
func MessageEvent(streamID uint, msg *protobuf.ChatMessage, awaitsApproval bool, authorID uint) *protobuf.RealtimeEvent {
	ev := Event(streamID)
	if awaitsApproval {
		ev.Audience = protobuf.RealtimeEvent_AUDIENCE_COURSE_ADMINS
		ev.AudienceUserId = uint32(authorID)
	}
	ev.Event = &protobuf.RealtimeEvent_Message{Message: &protobuf.ChatMessageEvent{Message: msg}}
	return ev
}

// ReactionsEvent carries a message's reactions in full. A message the viewers cannot
// see (an administrator reacting to a retracted one) keeps its reactions between the
// administrators.
func ReactionsEvent(streamID, messageID uint, reactions []*protobuf.ChatReactionSummary, adminsOnly bool) *protobuf.RealtimeEvent {
	ev := Event(streamID)
	if adminsOnly {
		ev.Audience = protobuf.RealtimeEvent_AUDIENCE_COURSE_ADMINS
	}
	ev.Event = &protobuf.RealtimeEvent_Reactions{Reactions: &protobuf.ChatReactionsEvent{
		MessageId: uint32(messageID),
		Reactions: reactions,
	}}
	return ev
}

// PollVoteEvent carries an option's vote count. Only the course's administrators see
// a running poll's tally, so that it does not sway the votes.
func PollVoteEvent(streamID, optionID uint, votes int64) *protobuf.RealtimeEvent {
	ev := Event(streamID)
	ev.Audience = protobuf.RealtimeEvent_AUDIENCE_COURSE_ADMINS
	ev.Event = &protobuf.RealtimeEvent_PollVote{PollVote: &protobuf.ChatPollVoteEvent{
		PollOptionId: uint32(optionID),
		Votes:        uint32(votes),
	}}
	return ev
}
