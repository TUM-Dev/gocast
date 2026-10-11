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

// restrictToHolders narrows an event about a message the viewers cannot see (one
// awaiting approval, or retracted) to the administrators and the author.
func restrictToHolders(ev *protobuf.RealtimeEvent, visible bool, authorID uint) {
	if !visible {
		ev.Audience = protobuf.RealtimeEvent_AUDIENCE_COURSE_ADMINS
		ev.AudienceUserId = uint32(authorID)
	}
}

// MessageDeletedEvent says a message and its replies are gone. One the viewers never
// saw is not announced to them.
func MessageDeletedEvent(streamID, messageID uint, visible bool, authorID uint) *protobuf.RealtimeEvent {
	ev := Event(streamID)
	restrictToHolders(ev, visible, authorID)
	ev.Event = &protobuf.RealtimeEvent_MessageDeleted{MessageDeleted: &protobuf.ChatMessageDeletedEvent{MessageId: uint32(messageID)}}
	return ev
}

// MessageResolvedEvent says a message was marked resolved; see MessageDeletedEvent for who hears it.
func MessageResolvedEvent(streamID, messageID uint, visible bool, authorID uint) *protobuf.RealtimeEvent {
	ev := Event(streamID)
	restrictToHolders(ev, visible, authorID)
	ev.Event = &protobuf.RealtimeEvent_MessageResolved{MessageResolved: &protobuf.ChatMessageResolvedEvent{MessageId: uint32(messageID)}}
	return ev
}

// MessageRetractedEvent says a message is hidden again. It goes to everyone: the
// viewers who had it drop it, and the holders keep it hidden-marked.
func MessageRetractedEvent(streamID, messageID uint) *protobuf.RealtimeEvent {
	ev := Event(streamID)
	ev.Event = &protobuf.RealtimeEvent_MessageRetracted{MessageRetracted: &protobuf.ChatMessageRetractedEvent{MessageId: uint32(messageID)}}
	return ev
}

// PollStartedEvent carries a new poll, without counts, to every viewer.
func PollStartedEvent(streamID uint, poll *protobuf.ChatPoll) *protobuf.RealtimeEvent {
	ev := Event(streamID)
	ev.Event = &protobuf.RealtimeEvent_PollStarted{PollStarted: &protobuf.ChatPollStartedEvent{Poll: poll}}
	return ev
}

// PollClosedEvent carries a closed poll with its final counts to every viewer.
func PollClosedEvent(streamID uint, poll *protobuf.ChatPoll) *protobuf.RealtimeEvent {
	ev := Event(streamID)
	ev.Event = &protobuf.RealtimeEvent_PollClosed{PollClosed: &protobuf.ChatPollClosedEvent{Poll: poll}}
	return ev
}
