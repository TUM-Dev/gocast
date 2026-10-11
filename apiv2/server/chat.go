package apiv2

import (
	"cmp"
	"context"
	"errors"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"google.golang.org/protobuf/types/known/timestamppb"
	"gorm.io/gorm"

	e "github.com/TUM-Dev/gocast/apiv2/errors"
	protobuf "github.com/TUM-Dev/gocast/apiv2/protobuf/server"
	"github.com/TUM-Dev/gocast/model"
)

// anonymousChatName is what the chat stores as the author's name on a message sent
// anonymously; the author's id is stored all the same.
const anonymousChatName = "Anonymous"

// chatReactionOrder is the order reactions are listed in, the order the picker
// offers them in. These are also the only emoji a message can be reacted with.
var chatReactionOrder = []string{"+1", "-1", "smile", "tada", "confused", "heart", "eyes"}

// ListChatMessages returns the stream's chat as the caller may see it: everything for
// the course's administrators, who moderate it, and otherwise the visible messages
// plus the caller's own, which they see while those await approval.
func (a *API) ListChatMessages(ctx context.Context, req *protobuf.ListChatMessagesRequest) (*protobuf.ListChatMessagesResponse, error) {
	user, stream, course, err := a.authorizeUserForStreamCourse(ctx, req)
	if err != nil {
		return nil, err
	}

	// 0 matches no author, so an anonymous caller gets the visible messages only.
	var uid uint
	if user != nil {
		uid = user.ID
	}

	var chats []model.Chat
	if user.CanAdminister(course) {
		chats, err = a.dao.GetAllChats(uid, stream.ID)
	} else {
		chats, err = a.dao.GetVisibleChats(uid, stream.ID)
	}
	if err != nil {
		a.log.Error("can't list chat messages", "err", err, "stream", stream.ID)
		return nil, e.WithStatus(http.StatusInternalServerError, errors.New("can't list chat messages"))
	}

	messages := make([]*protobuf.ChatMessage, 0, len(chats))
	for i := range chats {
		messages = append(messages, chatMessageToProto(&chats[i], uid))
	}

	return &protobuf.ListChatMessagesResponse{Messages: messages}, nil
}

// GetActivePoll returns the stream's running poll, if there is one. Vote counts are
// the course administrators' alone while it runs, so that the tally does not sway
// the votes; everyone sees them when the poll closes.
func (a *API) GetActivePoll(ctx context.Context, req *protobuf.GetActiveChatPollRequest) (*protobuf.GetActiveChatPollResponse, error) {
	user, stream, course, err := a.authorizeUserForStreamCourse(ctx, req)
	if err != nil {
		return nil, err
	}

	poll, err := a.dao.GetActivePoll(stream.ID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return &protobuf.GetActiveChatPollResponse{}, nil
	}
	if err != nil {
		a.log.Error("can't get active poll", "err", err, "stream", stream.ID)
		return nil, e.WithStatus(http.StatusInternalServerError, errors.New("can't get active poll"))
	}

	var voted uint
	if user != nil {
		if voted, err = a.dao.GetPollUserVote(poll.ID, user.ID); err != nil {
			a.log.Error("can't get the caller's poll vote", "err", err, "poll", poll.ID)
			return nil, e.WithStatus(http.StatusInternalServerError, errors.New("can't get active poll"))
		}
	}

	out, err := a.chatPollToProto(poll, user.CanAdminister(course))
	if err != nil {
		return nil, err
	}
	out.VotedOptionId = uint32(voted)

	return &protobuf.GetActiveChatPollResponse{Poll: out}, nil
}

// ListPolls returns the stream's closed polls with their results, for the course's
// administrators.
func (a *API) ListPolls(ctx context.Context, req *protobuf.ListChatPollsRequest) (*protobuf.ListChatPollsResponse, error) {
	user, stream, course, err := a.authorizeUserForStreamCourse(ctx, req)
	if err != nil {
		return nil, err
	}

	// The policy cannot say this: the request names a stream, not a course.
	if !user.CanAdminister(course) {
		return nil, e.WithStatus(http.StatusForbidden, errors.New("only the course's administrators can list its polls"))
	}

	polls, err := a.dao.GetPolls(stream.ID)
	if err != nil {
		a.log.Error("can't list polls", "err", err, "stream", stream.ID)
		return nil, e.WithStatus(http.StatusInternalServerError, errors.New("can't list polls"))
	}

	out := make([]*protobuf.ChatPoll, 0, len(polls))
	for _, poll := range polls {
		p, err := a.chatPollToProto(poll, true)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}

	return &protobuf.ListChatPollsResponse{Polls: out}, nil
}

// ListChatUsers returns who has written in the stream's chat under their own name,
// for @-mentions.
func (a *API) ListChatUsers(ctx context.Context, req *protobuf.ListChatUsersRequest) (*protobuf.ListChatUsersResponse, error) {
	_, stream, _, err := a.authorizeUserForStreamCourse(ctx, req)
	if err != nil {
		return nil, err
	}

	users, err := a.dao.GetChatUsers(stream.ID)
	if err != nil {
		a.log.Error("can't list chat users", "err", err, "stream", stream.ID)
		return nil, e.WithStatus(http.StatusInternalServerError, errors.New("can't list chat users"))
	}

	out := make([]*protobuf.ChatUser, 0, len(users))
	for i := range users {
		out = append(out, &protobuf.ChatUser{Id: uint32(users[i].ID), Name: users[i].GetPreferredName()})
	}

	return &protobuf.ListChatUsersResponse{Users: out}, nil
}

// chatPollToProto converts a poll, with its vote counts when withVotes.
func (a *API) chatPollToProto(poll model.Poll, withVotes bool) (*protobuf.ChatPoll, error) {
	options := make([]*protobuf.ChatPollOption, 0, len(poll.PollOptions))
	for _, option := range poll.PollOptions {
		var votes int64
		if withVotes {
			var err error
			if votes, err = a.dao.GetPollOptionVoteCount(option.ID); err != nil {
				a.log.Error("can't count poll votes", "err", err, "option", option.ID)
				return nil, e.WithStatus(http.StatusInternalServerError, errors.New("can't count poll votes"))
			}
		}
		options = append(options, &protobuf.ChatPollOption{
			Id:     uint32(option.ID),
			Answer: option.Answer,
			Votes:  uint32(votes),
		})
	}

	return &protobuf.ChatPoll{
		Id:       uint32(poll.ID),
		Question: poll.Question,
		Active:   poll.Active,
		Options:  options,
	}, nil
}

// chatMessageToProto converts a message and its replies for the user viewerID; 0
// for an anonymous viewer, or for an event that goes to everyone.
//
// An anonymous message keeps its author's id from everyone but the author. v1 sends
// it to every viewer, which makes the anonymity cosmetic.
func chatMessageToProto(chat *model.Chat, viewerID uint) *protobuf.ChatMessage {
	authorID, _ := strconv.ParseUint(chat.UserID, 10, 32)
	if chat.UserName == anonymousChatName && uint64(viewerID) != authorID {
		authorID = 0
	}

	var replyTo uint32
	if chat.ReplyTo.Valid {
		replyTo = uint32(chat.ReplyTo.Int64)
	}

	addressedTo := make([]uint32, 0, len(chat.AddressedToIds))
	for _, id := range chat.AddressedToIds {
		addressedTo = append(addressedTo, uint32(id))
	}

	replies := make([]*protobuf.ChatMessage, 0, len(chat.Replies))
	for i := range chat.Replies {
		replies = append(replies, chatMessageToProto(&chat.Replies[i], viewerID))
	}

	// The DAO sanitises on load; a message built in memory may not have been yet.
	if chat.SanitizedMessage == "" && chat.Message != "" {
		chat.SanitiseMessage()
	}

	return &protobuf.ChatMessage{
		Id:          uint32(chat.ID),
		UserId:      uint32(authorID),
		UserName:    chat.UserName,
		Color:       chat.Color,
		MessageHtml: chat.SanitizedMessage,
		MessageText: chat.Message,
		Admin:       chat.Admin,
		Visible:     chat.Visible.Bool,
		Resolved:    chat.Resolved,
		ReplyTo:     replyTo,
		AddressedTo: addressedTo,
		CreatedAt:   timestamppb.New(chat.CreatedAt),
		Reactions:   chatReactionsToProto(chat.Reactions),
		Replies:     replies,
	}
}

// chatReactionsToProto groups a message's reactions by emoji, in picker order.
func chatReactionsToProto(reactions []model.ChatReaction) []*protobuf.ChatReactionSummary {
	byEmoji := make(map[string]*protobuf.ChatReactionSummary)
	for _, r := range reactions {
		summary, ok := byEmoji[r.Emoji]
		if !ok {
			summary = &protobuf.ChatReactionSummary{Emoji: r.Emoji}
			byEmoji[r.Emoji] = summary
		}
		summary.Count++
		summary.UserIds = append(summary.UserIds, uint32(r.UserID))
		summary.UserNames = append(summary.UserNames, r.Username)
	}

	out := make([]*protobuf.ChatReactionSummary, 0, len(byEmoji))
	for _, summary := range byEmoji {
		out = append(out, summary)
	}

	// An emoji no longer offered sorts last rather than first, then by name.
	rank := func(emoji string) int {
		if i := slices.Index(chatReactionOrder, emoji); i >= 0 {
			return i
		}
		return len(chatReactionOrder)
	}
	slices.SortFunc(out, func(x, y *protobuf.ChatReactionSummary) int {
		return cmp.Or(cmp.Compare(rank(x.Emoji), rank(y.Emoji)), strings.Compare(x.Emoji, y.Emoji))
	})

	return out
}
