package api

import (
	"encoding/json"

	"github.com/gin-gonic/gin"

	"github.com/TUM-Dev/gocast/model"
)

// TEMPORARY, deleted with api/: chat writes made through the v2 API are repeated to
// the old UI's sockets, which are frozen and would otherwise stop seeing messages
// written from the SPA while both UIs are live. Each function is the broadcast v1's
// own handler makes for the same write, with the same payload.

// MirrorChatMessage sends a new message to the stream's old-UI viewers. One awaiting
// approval goes to the course's administrators and its author only, as in v1.
func MirrorChatMessage(streamID uint, chat model.Chat, awaitsApproval bool, authorID uint) {
	msg, err := json.Marshal(chat)
	if err != nil {
		return
	}
	if !awaitsApproval {
		broadcastStream(streamID, msg)
		return
	}
	for _, wrapper := range removeClosed(sessionsFor(streamID)) {
		if wrapper.isAdminOfCourse || wrapper.userID == authorID {
			_ = wrapper.session.Send(msg)
		}
	}
}

// MirrorChatReactions sends a message's reactions after a toggle.
func MirrorChatReactions(streamID, chatID uint, reactions []model.ChatReaction) {
	if msg, err := json.Marshal(gin.H{"reactions": chatID, "payload": reactions}); err == nil {
		broadcastStream(streamID, msg)
	}
}

// MirrorPollVote sends an option's vote count to the course's administrators.
func MirrorPollVote(streamID, optionID uint, votes int64) {
	msg, err := json.Marshal(gin.H{"type": POLL_PARTICIPATION_MSG, "pollOptionId": optionID, "votes": votes})
	if err == nil {
		broadcastStreamToAdmins(streamID, msg)
	}
}

// The moderation and poll writes. v1 broadcasts these to every session of the stream,
// held messages included; the mirror does the same.

// MirrorChatDeleted tells the old UI a message and its replies are gone.
func MirrorChatDeleted(streamID, chatID uint) {
	mirrorToStream(streamID, gin.H{"delete": chatID})
}

// MirrorChatResolved tells the old UI a message was marked resolved.
func MirrorChatResolved(streamID, chatID uint) {
	mirrorToStream(streamID, gin.H{"resolve": chatID})
}

// MirrorChatApproved sends an approved message, which the old UI shows from then on.
func MirrorChatApproved(streamID uint, chat model.Chat) {
	mirrorToStream(streamID, gin.H{"approve": chat.ID, "chat": chat})
}

// MirrorChatRetracted sends a retracted message, which the old UI hides again.
func MirrorChatRetracted(streamID uint, chat model.Chat) {
	mirrorToStream(streamID, gin.H{"retract": chat.ID, "chat": chat})
}

// MirrorPollStarted sends a new poll, with no votes yet.
func MirrorPollStarted(streamID uint, poll model.Poll) {
	mirrorToStream(streamID, pollStartedPayload(poll))
}

// pollStartedPayload is the frame v1's start_poll handler broadcasts.
func pollStartedPayload(poll model.Poll) gin.H {
	options := make([]gin.H, 0, len(poll.PollOptions))
	for _, option := range poll.PollOptions {
		options = append(options, option.GetStatsMap(0))
	}
	return gin.H{
		"type":      POLL_START_MSG,
		"active":    true,
		"question":  poll.Question,
		"options":   options,
		"submitted": 0,
	}
}

// MirrorPollClosed sends the final tally of a poll. votes maps an option's id to its count.
func MirrorPollClosed(streamID uint, poll model.Poll, votes map[uint]int64) {
	mirrorToStream(streamID, pollClosedPayload(poll, votes))
}

// pollClosedPayload is the frame v1's close_active_poll handler broadcasts.
func pollClosedPayload(poll model.Poll, votes map[uint]int64) gin.H {
	options := make([]gin.H, 0, len(poll.PollOptions))
	for _, option := range poll.PollOptions {
		options = append(options, option.GetStatsMap(votes[option.ID]))
	}
	return gin.H{"type": POLL_CLOSE_MSG, "question": poll.Question, "options": options}
}

func mirrorToStream(streamID uint, payload gin.H) {
	if msg, err := json.Marshal(payload); err == nil {
		broadcastStream(streamID, msg)
	}
}
