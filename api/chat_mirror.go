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
