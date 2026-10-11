package apiv2

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"slices"
	"strconv"

	"gorm.io/gorm"

	e "github.com/TUM-Dev/gocast/apiv2/errors"
	protobuf "github.com/TUM-Dev/gocast/apiv2/protobuf/server"
	"github.com/TUM-Dev/gocast/model"
	"github.com/TUM-Dev/gocast/pkg/realtimehub"
)

// maxChatMentions bounds the @-mentions of one message: each is a row, and nobody
// addresses a message to more people than this.
const maxChatMentions = 20

// authorizeChatWrite is authorizeUserForStreamCourse for the writes: the caller must
// be signed in, and the stream's chat must be on.
//
// Chat counts as on when either the course or the stream has it on, the rule v1
// applies to a message. The watch page shows the chat only when both do, so a
// stream could be written to that the page offers no chat for; that is v1's looseness
// kept, since the fixture and existing courses rely on it.
func (a *API) authorizeChatWrite(ctx context.Context, req StreamRequest) (*model.User, model.Stream, model.Course, error) {
	user, stream, course, err := a.authorizeUserForStreamCourse(ctx, req)
	if err != nil {
		return nil, stream, course, err
	}
	if user == nil {
		// The policy keeps anonymous callers out; this is the second lock.
		return nil, stream, course, e.WithStatus(http.StatusUnauthorized, errors.New("sign in to use the chat"))
	}
	if !course.ChatEnabled && !stream.ChatEnabled {
		return nil, stream, course, e.WithStatus(http.StatusForbidden, errors.New("the chat is not enabled for this stream"))
	}
	return user, stream, course, nil
}

// SendChatMessage writes a message in the stream's chat.
func (a *API) SendChatMessage(ctx context.Context, req *protobuf.SendChatMessageRequest) (*protobuf.SendChatMessageResponse, error) {
	user, stream, course, err := a.authorizeChatWrite(ctx, req)
	if err != nil {
		return nil, err
	}

	if req.Anonymous && !course.AnonymousChatEnabled {
		return nil, e.WithStatus(http.StatusBadRequest, errors.New("this course does not allow anonymous messages"))
	}
	if len(req.AddressedTo) > maxChatMentions {
		return nil, e.WithStatus(http.StatusBadRequest, errors.New("too many mentions"))
	}

	isAdmin := user.CanAdminister(course)

	replyTo := sql.NullInt64{}
	if req.ReplyTo != nil && *req.ReplyTo != 0 {
		parent, err := a.dao.GetChat(uint(*req.ReplyTo), user.ID)
		if err != nil {
			a.log.Error("can't load the message replied to", "err", err, "message", *req.ReplyTo)
			return nil, e.WithStatus(http.StatusInternalServerError, errors.New("can't send the message"))
		}
		// Find reports a missing row as a zero value. A message the caller cannot see
		// answers like one that does not exist; it must not confirm the id.
		if parent.ID == 0 || (!parent.Visible.Bool && !isAdmin && parent.UserID != strconv.Itoa(int(user.ID))) {
			return nil, e.WithStatus(http.StatusNotFound, model.ErrReplyToNoMsg)
		}
		replyTo = sql.NullInt64{Int64: int64(*req.ReplyTo), Valid: true}
	}

	name := user.GetPreferredName()
	if req.Anonymous {
		name = anonymousChatName
	}

	// A moderated chat holds an ordinary user's messages until an administrator
	// approves them.
	awaitsApproval := course.ModeratedChatEnabled && !isAdmin
	visible := sql.NullBool{Valid: true, Bool: !awaitsApproval}

	chat := model.Chat{
		UserID:         strconv.FormatUint(uint64(user.ID), 10),
		UserName:       name,
		Message:        req.Text,
		StreamID:       stream.ID,
		Admin:          isAdmin,
		ReplyTo:        replyTo,
		Visible:        visible,
		IsVisible:      visible.Bool,
		AddressedToIds: mentionIDs(req.AddressedTo),
	}
	if err := a.dao.AddMessage(&chat); err != nil {
		return nil, chatWriteError(err, a, "can't send the message")
	}
	chat.SanitiseMessage()

	// The event goes to every viewer, so it carries no viewer's view: viewer 0 keeps
	// an anonymous author's id out of it. The response is the author's own.
	a.publish(ctx, stream.ID, realtimehub.MessageEvent(stream.ID, chatMessageToProto(&chat, 0), awaitsApproval, user.ID))

	return &protobuf.SendChatMessageResponse{Message: chatMessageToProto(&chat, user.ID)}, nil
}

// mentionIDs drops zero ids and repeats from a message's mentions.
func mentionIDs(ids []uint32) []uint {
	out := make([]uint, 0, len(ids))
	for _, id := range ids {
		if id != 0 && !slices.Contains(out, uint(id)) {
			out = append(out, uint(id))
		}
	}
	return out
}

// chatWriteError turns what refused a message into the status the caller should see.
func chatWriteError(err error, a *API, fallback string) error {
	switch {
	case errors.Is(err, model.ErrCooledDown):
		return e.WithStatus(http.StatusTooManyRequests, errors.New("you are sending messages too fast, please wait a bit"))
	case errors.Is(err, model.ErrMessageTooLong), errors.Is(err, model.ErrMessageNoText),
		errors.Is(err, model.ErrReplyToReply), errors.Is(err, model.ErrReplyToWrongStream):
		return e.WithStatus(http.StatusBadRequest, err)
	case errors.Is(err, model.ErrReplyToNoMsg):
		return e.WithStatus(http.StatusNotFound, err)
	}
	a.log.Error(fallback, "err", err)
	return e.WithStatus(http.StatusInternalServerError, errors.New(fallback))
}

// ToggleChatReaction adds the caller's reaction to a message, or removes it when they
// have reacted with that emoji already.
func (a *API) ToggleChatReaction(ctx context.Context, req *protobuf.ToggleChatReactionRequest) (*protobuf.ToggleChatReactionResponse, error) {
	user, stream, course, err := a.authorizeChatWrite(ctx, req)
	if err != nil {
		return nil, err
	}

	if !slices.Contains(chatReactionOrder, req.Emoji) {
		return nil, e.WithStatus(http.StatusBadRequest, errors.New("that emoji is not one of the reactions"))
	}

	chat, err := a.dao.GetChat(uint(req.MessageId), user.ID)
	if err != nil {
		a.log.Error("can't load the message to react to", "err", err, "message", req.MessageId)
		return nil, e.WithStatus(http.StatusInternalServerError, errors.New("can't react to the message"))
	}
	isAdmin := user.CanAdminister(course)
	// A message of another stream, or one the caller cannot see, is no message here.
	if chat.ID == 0 || chat.StreamID != stream.ID || (!chat.Visible.Bool && !isAdmin) {
		return nil, e.WithStatus(http.StatusNotFound, errors.New("no such message"))
	}

	if err := a.dao.ToggleReaction(user.ID, chat.ID, user.Name, req.Emoji); err != nil {
		a.log.Error("can't toggle the reaction", "err", err, "message", chat.ID)
		return nil, e.WithStatus(http.StatusInternalServerError, errors.New("can't react to the message"))
	}
	reactions, err := a.dao.GetReactions(chat.ID)
	if err != nil {
		a.log.Error("can't load the reactions", "err", err, "message", chat.ID)
		return nil, e.WithStatus(http.StatusInternalServerError, errors.New("can't react to the message"))
	}

	summary := chatReactionsToProto(reactions)
	adminsOnly := !chat.Visible.Bool
	a.publish(ctx, stream.ID, realtimehub.ReactionsEvent(stream.ID, chat.ID, summary, adminsOnly))

	return &protobuf.ToggleChatReactionResponse{Reactions: summary}, nil
}

// VotePoll records the caller's vote in the stream's running poll. A vote is final.
func (a *API) VotePoll(ctx context.Context, req *protobuf.VoteChatPollRequest) (*protobuf.VoteChatPollResponse, error) {
	user, stream, course, err := a.authorizeChatWrite(ctx, req)
	if err != nil {
		return nil, err
	}

	poll, err := a.dao.GetActivePoll(stream.ID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, e.WithStatus(http.StatusNotFound, errors.New("no poll is running"))
	}
	if err != nil {
		a.log.Error("can't get active poll", "err", err, "stream", stream.ID)
		return nil, e.WithStatus(http.StatusInternalServerError, errors.New("can't vote"))
	}

	// Only the running poll's own options: v1 took any option id, of any stream and
	// of a poll long closed. A closed poll's options are unknown here too.
	if !slices.ContainsFunc(poll.PollOptions, func(o model.PollOption) bool { return o.ID == uint(req.PollOptionId) }) {
		return nil, e.WithStatus(http.StatusNotFound, errors.New("that option is not part of the running poll"))
	}

	voted, err := a.dao.GetPollUserVote(poll.ID, user.ID)
	if err != nil {
		a.log.Error("can't get the caller's poll vote", "err", err, "poll", poll.ID)
		return nil, e.WithStatus(http.StatusInternalServerError, errors.New("can't vote"))
	}
	if voted != 0 {
		return nil, e.WithStatus(http.StatusConflict, errors.New("you have voted in this poll already"))
	}

	if err := a.dao.AddChatPollOptionVote(uint(req.PollOptionId), user.ID); err != nil {
		a.log.Error("can't add the vote", "err", err, "poll", poll.ID)
		return nil, e.WithStatus(http.StatusInternalServerError, errors.New("can't vote"))
	}

	votes, err := a.dao.GetPollOptionVoteCount(uint(req.PollOptionId))
	if err != nil {
		a.log.Error("can't count the votes", "err", err, "option", req.PollOptionId)
		return nil, e.WithStatus(http.StatusInternalServerError, errors.New("can't vote"))
	}
	a.publish(ctx, stream.ID, realtimehub.PollVoteEvent(stream.ID, uint(req.PollOptionId), votes))

	out, err := a.chatPollToProto(poll, user.CanAdminister(course))
	if err != nil {
		return nil, err
	}
	out.VotedOptionId = req.PollOptionId

	return &protobuf.VoteChatPollResponse{Poll: out}, nil
}
