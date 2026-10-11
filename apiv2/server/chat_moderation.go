package apiv2

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"google.golang.org/protobuf/types/known/emptypb"
	"gorm.io/gorm"

	e "github.com/TUM-Dev/gocast/apiv2/errors"
	protobuf "github.com/TUM-Dev/gocast/apiv2/protobuf/server"
	"github.com/TUM-Dev/gocast/model"
	"github.com/TUM-Dev/gocast/pkg/realtimehub"
)

// No v1 mirror, by decision: the watch pages and the old chat UI are replaced together,
// so these writes are not repeated to the old sockets.

const (
	maxPollQuestion = 500
	maxPollAnswer   = 200
	minPollAnswers  = 2
	maxPollAnswers  = 10
)

// authorizeChatModeration is authorizeUserForStreamCourse for the calls only the
// course's administrators make. The policy cannot say this: the requests name a
// stream, not a course, so the course is the stream's and is found here. That also
// makes "the stream belongs to the course" true by construction, which v1 never
// checked of the message it was handed.
//
// The chat need not be on: an administrator may still tidy up a chat that was switched off.
func (a *API) authorizeChatModeration(ctx context.Context, req StreamRequest) (*model.User, model.Stream, error) {
	user, stream, course, err := a.authorizeUserForStreamCourse(ctx, req)
	if err != nil {
		return nil, stream, err
	}
	if user == nil {
		return nil, stream, e.WithStatus(http.StatusUnauthorized, errors.New("sign in to moderate the chat"))
	}
	if !user.CanAdminister(course) {
		return nil, stream, e.WithStatus(http.StatusForbidden, errors.New("only the course's administrators can moderate its chat"))
	}
	return user, stream, nil
}

// moderatedMessage loads a message of the stream for a moderation call. A message of
// another stream is no message here, v1 acted on any id it was given.
func (a *API) moderatedMessage(streamID uint, messageID uint32) (*model.Chat, error) {
	chat, err := a.dao.GetChat(uint(messageID), 0)
	if err != nil {
		a.log.Error("can't load the message to moderate", "err", err, "message", messageID)
		return nil, e.WithStatus(http.StatusInternalServerError, errors.New("can't load the message"))
	}
	if chat.ID == 0 || chat.StreamID != streamID {
		return nil, e.WithStatus(http.StatusNotFound, errors.New("no such message"))
	}
	return chat, nil
}

func chatAuthorID(chat *model.Chat) uint {
	id, _ := strconv.ParseUint(chat.UserID, 10, 32)
	return uint(id)
}

// visibleReplies drops the replies the viewers cannot see from a message that is
// about to be sent to all of them.
func visibleReplies(chat model.Chat) model.Chat {
	kept := make([]model.Chat, 0, len(chat.Replies))
	for _, r := range chat.Replies {
		if r.Visible.Bool {
			kept = append(kept, r)
		}
	}
	chat.Replies = kept
	return chat
}

// ApproveChatMessage publishes a message that awaited approval to every viewer.
func (a *API) ApproveChatMessage(ctx context.Context, req *protobuf.ApproveChatMessageRequest) (*protobuf.ApproveChatMessageResponse, error) {
	user, stream, err := a.authorizeChatModeration(ctx, req)
	if err != nil {
		return nil, err
	}
	chat, err := a.moderatedMessage(stream.ID, req.MessageId)
	if err != nil {
		return nil, err
	}

	if err := a.dao.ApproveChat(chat.ID); err != nil {
		a.log.Error("can't approve the message", "err", err, "message", chat.ID)
		return nil, e.WithStatus(http.StatusInternalServerError, errors.New("can't approve the message"))
	}
	chat.Visible.Bool, chat.Visible.Valid, chat.IsVisible = true, true, true
	chat.Retracted = false

	// No viewer's view (0): the message goes to everyone, and nobody has reacted to it yet.
	toAll := visibleReplies(*chat)
	a.publish(ctx, stream.ID, realtimehub.MessageEvent(stream.ID, chatMessageToProto(&toAll, 0), false, 0))

	return &protobuf.ApproveChatMessageResponse{Message: chatMessageToProto(chat, user.ID)}, nil
}

// RetractChatMessage hides an approved message again and drops its reactions.
func (a *API) RetractChatMessage(ctx context.Context, req *protobuf.RetractChatMessageRequest) (*protobuf.RetractChatMessageResponse, error) {
	user, stream, err := a.authorizeChatModeration(ctx, req)
	if err != nil {
		return nil, err
	}
	chat, err := a.moderatedMessage(stream.ID, req.MessageId)
	if err != nil {
		return nil, err
	}

	if err := a.dao.RetractChat(chat.ID); err != nil {
		a.log.Error("can't retract the message", "err", err, "message", chat.ID)
		return nil, e.WithStatus(http.StatusInternalServerError, errors.New("can't retract the message"))
	}
	if err := a.dao.RemoveReactions(chat.ID); err != nil {
		a.log.Error("can't remove the reactions of the retracted message", "err", err, "message", chat.ID)
		return nil, e.WithStatus(http.StatusInternalServerError, errors.New("can't retract the message"))
	}
	chat.Visible.Bool, chat.Visible.Valid, chat.IsVisible = false, true, false
	chat.Retracted = true
	chat.Reactions = nil

	a.publish(ctx, stream.ID, realtimehub.MessageRetractedEvent(stream.ID, chat.ID))

	return &protobuf.RetractChatMessageResponse{Message: chatMessageToProto(chat, user.ID)}, nil
}

// DeleteChatMessage removes a message, and its replies, for everyone.
func (a *API) DeleteChatMessage(ctx context.Context, req *protobuf.DeleteChatMessageRequest) (*emptypb.Empty, error) {
	_, stream, err := a.authorizeChatModeration(ctx, req)
	if err != nil {
		return nil, err
	}
	chat, err := a.moderatedMessage(stream.ID, req.MessageId)
	if err != nil {
		return nil, err
	}

	if err := a.dao.DeleteChat(chat.ID); err != nil {
		a.log.Error("can't delete the message", "err", err, "message", chat.ID)
		return nil, e.WithStatus(http.StatusInternalServerError, errors.New("can't delete the message"))
	}

	a.publish(ctx, stream.ID, realtimehub.MessageDeletedEvent(stream.ID, chat.ID, chat.Visible.Bool, chatAuthorID(chat)))

	return &emptypb.Empty{}, nil
}

// ResolveChatMessage marks a message, usually a question, as resolved.
func (a *API) ResolveChatMessage(ctx context.Context, req *protobuf.ResolveChatMessageRequest) (*protobuf.ResolveChatMessageResponse, error) {
	user, stream, err := a.authorizeChatModeration(ctx, req)
	if err != nil {
		return nil, err
	}
	chat, err := a.moderatedMessage(stream.ID, req.MessageId)
	if err != nil {
		return nil, err
	}

	if err := a.dao.ResolveChat(chat.ID); err != nil {
		a.log.Error("can't resolve the message", "err", err, "message", chat.ID)
		return nil, e.WithStatus(http.StatusInternalServerError, errors.New("can't resolve the message"))
	}
	chat.Resolved = true

	a.publish(ctx, stream.ID, realtimehub.MessageResolvedEvent(stream.ID, chat.ID, chat.Visible.Bool, chatAuthorID(chat)))

	return &protobuf.ResolveChatMessageResponse{Message: chatMessageToProto(chat, user.ID)}, nil
}

// StartChatPoll starts a poll, when none is running.
func (a *API) StartChatPoll(ctx context.Context, req *protobuf.StartChatPollRequest) (*protobuf.StartChatPollResponse, error) {
	_, stream, err := a.authorizeChatModeration(ctx, req)
	if err != nil {
		return nil, err
	}

	question := strings.TrimSpace(req.Question)
	if question == "" || utf8.RuneCountInString(question) > maxPollQuestion {
		return nil, e.WithStatus(http.StatusBadRequest, errors.New("a poll needs a question of up to 500 characters"))
	}
	if len(req.Answers) < minPollAnswers || len(req.Answers) > maxPollAnswers {
		return nil, e.WithStatus(http.StatusBadRequest, errors.New("a poll needs two to ten answers"))
	}
	options := make([]model.PollOption, 0, len(req.Answers))
	for _, answer := range req.Answers {
		answer = strings.TrimSpace(answer)
		if answer == "" || utf8.RuneCountInString(answer) > maxPollAnswer {
			return nil, e.WithStatus(http.StatusBadRequest, errors.New("every answer needs text of up to 200 characters"))
		}
		options = append(options, model.PollOption{Answer: answer})
	}

	// v1 started a second poll beside the first, after which neither could be told
	// from the other: the chat shows one, and votes land in whichever the id names.
	switch _, err := a.dao.GetActivePoll(stream.ID); {
	case err == nil:
		return nil, e.WithStatus(http.StatusConflict, errors.New("a poll is running already; close it first"))
	case !errors.Is(err, gorm.ErrRecordNotFound):
		a.log.Error("can't look for a running poll", "err", err, "stream", stream.ID)
		return nil, e.WithStatus(http.StatusInternalServerError, errors.New("can't start the poll"))
	}

	poll := model.Poll{StreamID: stream.ID, Question: question, Active: true, PollOptions: options}
	if err := a.dao.AddChatPoll(&poll); err != nil {
		a.log.Error("can't start the poll", "err", err, "stream", stream.ID)
		return nil, e.WithStatus(http.StatusInternalServerError, errors.New("can't start the poll"))
	}

	// Nobody has voted, so every viewer's view of the new poll is the same.
	out, err := a.chatPollToProto(poll, false)
	if err != nil {
		return nil, err
	}
	a.publish(ctx, stream.ID, realtimehub.PollStartedEvent(stream.ID, out))

	return &protobuf.StartChatPollResponse{Poll: out}, nil
}

// CloseChatPoll closes the running poll and shows everyone the result.
func (a *API) CloseChatPoll(ctx context.Context, req *protobuf.CloseChatPollRequest) (*protobuf.CloseChatPollResponse, error) {
	_, stream, err := a.authorizeChatModeration(ctx, req)
	if err != nil {
		return nil, err
	}

	poll, err := a.dao.GetActivePoll(stream.ID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, e.WithStatus(http.StatusNotFound, errors.New("no poll is running"))
	}
	if err != nil {
		a.log.Error("can't get active poll", "err", err, "stream", stream.ID)
		return nil, e.WithStatus(http.StatusInternalServerError, errors.New("can't close the poll"))
	}

	// Counted before closing: a failure here leaves the poll running to be closed again.
	out, err := a.chatPollToProto(poll, true)
	if err != nil {
		return nil, err
	}
	if err := a.dao.CloseActivePoll(stream.ID); err != nil {
		a.log.Error("can't close the poll", "err", err, "stream", stream.ID)
		return nil, e.WithStatus(http.StatusInternalServerError, errors.New("can't close the poll"))
	}
	out.Active = false

	a.publish(ctx, stream.ID, realtimehub.PollClosedEvent(stream.ID, out))

	return &protobuf.CloseChatPollResponse{Poll: out}, nil
}
