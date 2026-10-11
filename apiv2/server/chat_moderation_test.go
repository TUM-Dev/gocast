package apiv2

import (
	"database/sql"
	"slices"
	"strings"
	"testing"

	"go.uber.org/mock/gomock"
	"google.golang.org/grpc/codes"
	"gorm.io/gorm"

	protobuf "github.com/TUM-Dev/gocast/apiv2/protobuf/server"
	"github.com/TUM-Dev/gocast/model"
)

// Another course's administrator: signed in, an administrator, but not of course 5.
var chatOtherAdmin = &model.User{
	Model:               gorm.Model{ID: 10},
	Name:                "Anderer Dozent",
	AdministeredCourses: []model.Course{{Model: gorm.Model{ID: 6}}},
}

var moderationMethods = []string{
	"approveChatMessage", "retractChatMessage", "deleteChatMessage",
	"resolveChatMessage", "startChatPoll", "closeChatPoll",
}

func TestChatModerationIsPolicedAsSignedInAndChecksTheCourseItself(t *testing.T) {
	for _, name := range moderationMethods {
		p, ok := methodPolicies["/protobuf.ChatService/"+name]
		if !ok {
			t.Fatalf("%s has no policy", name)
		}
		// requiresCourseAdmin reads a course_id the requests do not have; the
		// handlers find the course from the stream, which TestChatModerationRefuses...
		// proves for each of them.
		if p != authenticated {
			t.Errorf("%s policy = %+v, want authenticated", name, p)
		}
	}
}

// moderationCalls runs every moderation RPC as `user` against stream 7 and message 30.
func moderationCalls(f chatWriteFixture, user *model.User) map[string]error {
	ctx := asCaller(user)
	out := map[string]error{}
	_, out["approve"] = f.api.ApproveChatMessage(ctx, &protobuf.ApproveChatMessageRequest{StreamId: 7, MessageId: 30})
	_, out["retract"] = f.api.RetractChatMessage(ctx, &protobuf.RetractChatMessageRequest{StreamId: 7, MessageId: 30})
	_, out["delete"] = f.api.DeleteChatMessage(ctx, &protobuf.DeleteChatMessageRequest{StreamId: 7, MessageId: 30})
	_, out["resolve"] = f.api.ResolveChatMessage(ctx, &protobuf.ResolveChatMessageRequest{StreamId: 7, MessageId: 30})
	_, out["start"] = f.api.StartChatPoll(ctx, &protobuf.StartChatPollRequest{StreamId: 7, Question: "Q?", Answers: []string{"a", "b"}})
	_, out["close"] = f.api.CloseChatPoll(ctx, &protobuf.CloseChatPollRequest{StreamId: 7})
	return out
}

func TestChatModerationRefuses(t *testing.T) {
	// No expectation is set on the ChatDao: a refused call must not reach it.
	for name, tc := range map[string]struct {
		user *model.User
		opts chatWriteOpts
		want codes.Code
	}{
		"a student":                        {chatStudent, chatWriteOpts{courseChat: true}, codes.PermissionDenied},
		"another course's administrator":   {chatOtherAdmin, chatWriteOpts{courseChat: true}, codes.PermissionDenied},
		"an anonymous caller":              {nil, chatWriteOpts{courseChat: true, visibility: "public"}, codes.Unauthenticated},
		"a user the course does not admit": {chatStudent, chatWriteOpts{courseChat: true, visibility: "enrolled"}, codes.PermissionDenied},
	} {
		t.Run(name, func(t *testing.T) {
			f := newChatWriteFixture(t, tc.opts)
			for call, err := range moderationCalls(f, tc.user) {
				if err == nil {
					t.Errorf("%s succeeded", call)
					continue
				}
				wantChatCode(t, err, tc.want)
			}
			f.none(t)
		})
	}
}

func TestChatModerationOfAMessage(t *testing.T) {
	held := func() *model.Chat {
		return &model.Chat{Model: gorm.Model{ID: 30}, StreamID: 7, UserID: "3", UserName: "Studi", Message: "q?", Visible: sql.NullBool{Valid: true}}
	}
	approved := func() *model.Chat {
		c := held()
		c.Visible.Bool = true
		return c
	}

	t.Run("a message of another stream, or none, is a 404", func(t *testing.T) {
		for name, chat := range map[string]*model.Chat{
			"another stream": {Model: gorm.Model{ID: 30}, StreamID: 8, Visible: sql.NullBool{Valid: true, Bool: true}},
			"missing":        {},
		} {
			t.Run(name, func(t *testing.T) {
				f := newChatWriteFixture(t, chatWriteOpts{courseChat: true})
				f.chat.EXPECT().GetChat(uint(30), uint(0)).Return(chat, nil).Times(4)
				ctx := asCaller(chatAdmin)

				_, err := f.api.ApproveChatMessage(ctx, &protobuf.ApproveChatMessageRequest{StreamId: 7, MessageId: 30})
				wantChatCode(t, err, codes.NotFound)
				_, err = f.api.RetractChatMessage(ctx, &protobuf.RetractChatMessageRequest{StreamId: 7, MessageId: 30})
				wantChatCode(t, err, codes.NotFound)
				_, err = f.api.DeleteChatMessage(ctx, &protobuf.DeleteChatMessageRequest{StreamId: 7, MessageId: 30})
				wantChatCode(t, err, codes.NotFound)
				_, err = f.api.ResolveChatMessage(ctx, &protobuf.ResolveChatMessageRequest{StreamId: 7, MessageId: 30})
				wantChatCode(t, err, codes.NotFound)
				f.none(t)
			})
		}
	})

	t.Run("approving publishes the message to everyone, replies awaiting approval left out", func(t *testing.T) {
		f := newChatWriteFixture(t, chatWriteOpts{courseChat: true, moderated: true})
		chat := held()
		chat.Replies = []model.Chat{
			{Model: gorm.Model{ID: 31}, StreamID: 7, ReplyTo: sql.NullInt64{Valid: true, Int64: 30}, Message: "seen", Visible: sql.NullBool{Valid: true, Bool: true}},
			{Model: gorm.Model{ID: 32}, StreamID: 7, ReplyTo: sql.NullInt64{Valid: true, Int64: 30}, Message: "held", Visible: sql.NullBool{Valid: true}},
		}
		f.chat.EXPECT().GetChat(uint(30), uint(0)).Return(chat, nil)
		f.chat.EXPECT().ApproveChat(uint(30)).Return(nil)

		resp, err := f.api.ApproveChatMessage(asCaller(chatAdmin), &protobuf.ApproveChatMessageRequest{StreamId: 7, MessageId: 30})
		if err != nil {
			t.Fatalf("ApproveChatMessage: %v", err)
		}
		if !resp.Message.Visible || resp.Message.Retracted {
			t.Errorf("response = %v, want visible and no longer retracted", resp.Message)
		}
		ev := f.next(t)
		got := ev.GetMessage().GetMessage()
		if ev.Audience != protobuf.RealtimeEvent_AUDIENCE_ALL || got.GetId() != 30 || !got.GetVisible() {
			t.Errorf("event = %v, want message 30, visible, for everyone", ev)
		}
		if len(got.Replies) != 1 || got.Replies[0].Id != 31 {
			t.Errorf("replies = %v, want only the visible one", got.Replies)
		}
	})

	t.Run("retracting hides the message, drops its reactions and tells everyone", func(t *testing.T) {
		f := newChatWriteFixture(t, chatWriteOpts{courseChat: true})
		chat := approved()
		chat.Reactions = []model.ChatReaction{{ChatID: 30, UserID: 4, Emoji: "tada"}}
		f.chat.EXPECT().GetChat(uint(30), uint(0)).Return(chat, nil)
		f.chat.EXPECT().RetractChat(uint(30)).Return(nil)
		f.chat.EXPECT().RemoveReactions(uint(30)).Return(nil)

		resp, err := f.api.RetractChatMessage(asCaller(chatAdmin), &protobuf.RetractChatMessageRequest{StreamId: 7, MessageId: 30})
		if err != nil {
			t.Fatalf("RetractChatMessage: %v", err)
		}
		if resp.Message.Visible || !resp.Message.Retracted || len(resp.Message.Reactions) != 0 {
			t.Errorf("response = %v, want hidden, retracted, no reactions", resp.Message)
		}
		ev := f.next(t)
		if ev.Audience != protobuf.RealtimeEvent_AUDIENCE_ALL || ev.GetMessageRetracted().GetMessageId() != 30 {
			t.Errorf("event = %v", ev)
		}
	})

	t.Run("deleting a visible message tells everyone", func(t *testing.T) {
		f := newChatWriteFixture(t, chatWriteOpts{courseChat: true})
		f.chat.EXPECT().GetChat(uint(30), uint(0)).Return(approved(), nil)
		f.chat.EXPECT().DeleteChat(uint(30)).Return(nil)

		if _, err := f.api.DeleteChatMessage(asCaller(chatAdmin), &protobuf.DeleteChatMessageRequest{StreamId: 7, MessageId: 30}); err != nil {
			t.Fatalf("DeleteChatMessage: %v", err)
		}
		ev := f.next(t)
		if ev.Audience != protobuf.RealtimeEvent_AUDIENCE_ALL || ev.GetMessageDeleted().GetMessageId() != 30 {
			t.Errorf("event = %v", ev)
		}
	})

	t.Run("deleting a message awaiting approval tells the administrators and its author only", func(t *testing.T) {
		f := newChatWriteFixture(t, chatWriteOpts{courseChat: true, moderated: true})
		f.chat.EXPECT().GetChat(uint(30), uint(0)).Return(held(), nil)
		f.chat.EXPECT().DeleteChat(uint(30)).Return(nil)

		if _, err := f.api.DeleteChatMessage(asCaller(chatAdmin), &protobuf.DeleteChatMessageRequest{StreamId: 7, MessageId: 30}); err != nil {
			t.Fatalf("DeleteChatMessage: %v", err)
		}
		ev := f.next(t)
		if ev.Audience != protobuf.RealtimeEvent_AUDIENCE_COURSE_ADMINS || ev.AudienceUserId != 3 {
			t.Errorf("event = %v, want the admins and user 3", ev)
		}
	})

	t.Run("resolving marks the message and tells everyone", func(t *testing.T) {
		f := newChatWriteFixture(t, chatWriteOpts{courseChat: true})
		f.chat.EXPECT().GetChat(uint(30), uint(0)).Return(approved(), nil)
		f.chat.EXPECT().ResolveChat(uint(30)).Return(nil)

		resp, err := f.api.ResolveChatMessage(asCaller(chatAdmin), &protobuf.ResolveChatMessageRequest{StreamId: 7, MessageId: 30})
		if err != nil {
			t.Fatalf("ResolveChatMessage: %v", err)
		}
		if !resp.Message.Resolved {
			t.Errorf("response = %v", resp.Message)
		}
		ev := f.next(t)
		if ev.Audience != protobuf.RealtimeEvent_AUDIENCE_ALL || ev.GetMessageResolved().GetMessageId() != 30 {
			t.Errorf("event = %v", ev)
		}
	})

	t.Run("a database failure publishes nothing", func(t *testing.T) {
		f := newChatWriteFixture(t, chatWriteOpts{courseChat: true})
		f.chat.EXPECT().GetChat(uint(30), uint(0)).Return(approved(), nil)
		f.chat.EXPECT().DeleteChat(uint(30)).Return(gorm.ErrInvalidDB)

		_, err := f.api.DeleteChatMessage(asCaller(chatAdmin), &protobuf.DeleteChatMessageRequest{StreamId: 7, MessageId: 30})

		wantChatCode(t, err, codes.Unknown) // a 500 maps to Unknown, see e.WithStatus
		f.none(t)
	})
}

func TestStartChatPoll(t *testing.T) {
	start := func(f chatWriteFixture, req *protobuf.StartChatPollRequest) (*protobuf.StartChatPollResponse, error) {
		req.StreamId = 7
		return f.api.StartChatPoll(asCaller(chatAdmin), req)
	}

	t.Run("stores the poll and tells everyone, without counts", func(t *testing.T) {
		f := newChatWriteFixture(t, chatWriteOpts{courseChat: true})
		f.chat.EXPECT().GetActivePoll(uint(7)).Return(model.Poll{}, gorm.ErrRecordNotFound)
		f.chat.EXPECT().AddChatPoll(gomock.Any()).DoAndReturn(func(p *model.Poll) error {
			if p.StreamID != 7 || !p.Active || p.Question != "Q?" || len(p.PollOptions) != 2 || p.PollOptions[0].Answer != "yes" {
				t.Errorf("stored %+v", p)
			}
			p.ID = 60
			p.PollOptions[0].ID, p.PollOptions[1].ID = 601, 602
			return nil
		})

		resp, err := start(f, &protobuf.StartChatPollRequest{Question: "  Q?  ", Answers: []string{"yes", " no "}})
		if err != nil {
			t.Fatalf("StartChatPoll: %v", err)
		}
		if resp.Poll.Id != 60 || !resp.Poll.Active || len(resp.Poll.Options) != 2 || resp.Poll.Options[1].Answer != "no" {
			t.Errorf("response = %v", resp.Poll)
		}
		ev := f.next(t)
		if ev.Audience != protobuf.RealtimeEvent_AUDIENCE_ALL || ev.GetPollStarted().GetPoll().GetId() != 60 {
			t.Errorf("event = %v", ev)
		}
	})

	t.Run("a poll already running is a conflict", func(t *testing.T) {
		f := newChatWriteFixture(t, chatWriteOpts{courseChat: true})
		f.chat.EXPECT().GetActivePoll(uint(7)).Return(model.Poll{Model: gorm.Model{ID: 60}, Active: true}, nil)

		_, err := start(f, &protobuf.StartChatPollRequest{Question: "Q?", Answers: []string{"a", "b"}})

		wantChatCode(t, err, codes.AlreadyExists)
		f.none(t)
	})

	t.Run("refuses a poll without a question or with too few, too many or empty answers", func(t *testing.T) {
		many := slices.Repeat([]string{"x"}, 11)
		for name, req := range map[string]*protobuf.StartChatPollRequest{
			"no question":  {Question: "  ", Answers: []string{"a", "b"}},
			"one answer":   {Question: "Q?", Answers: []string{"a"}},
			"no answers":   {Question: "Q?"},
			"empty answer": {Question: "Q?", Answers: []string{"a", " "}},
			"eleven":       {Question: "Q?", Answers: many},
			"long answer":  {Question: "Q?", Answers: []string{"a", strings.Repeat("x", 201)}},
		} {
			t.Run(name, func(t *testing.T) {
				f := newChatWriteFixture(t, chatWriteOpts{courseChat: true})
				_, err := start(f, req)
				wantChatCode(t, err, codes.InvalidArgument)
				f.none(t)
			})
		}
	})
}

func TestCloseChatPoll(t *testing.T) {
	poll := model.Poll{
		Model: gorm.Model{ID: 60}, StreamID: 7, Question: "Q?", Active: true,
		PollOptions: []model.PollOption{{Model: gorm.Model{ID: 601}, Answer: "yes"}, {Model: gorm.Model{ID: 602}, Answer: "no"}},
	}

	t.Run("closes the poll and shows everyone the result", func(t *testing.T) {
		f := newChatWriteFixture(t, chatWriteOpts{courseChat: true})
		f.chat.EXPECT().GetActivePoll(uint(7)).Return(poll, nil)
		f.chat.EXPECT().GetPollOptionVoteCount(uint(601)).Return(int64(5), nil)
		f.chat.EXPECT().GetPollOptionVoteCount(uint(602)).Return(int64(2), nil)
		f.chat.EXPECT().CloseActivePoll(uint(7)).Return(nil)

		resp, err := f.api.CloseChatPoll(asCaller(chatAdmin), &protobuf.CloseChatPollRequest{StreamId: 7})
		if err != nil {
			t.Fatalf("CloseChatPoll: %v", err)
		}
		if resp.Poll.Active || resp.Poll.Options[0].Votes != 5 || resp.Poll.Options[1].Votes != 2 {
			t.Errorf("response = %v", resp.Poll)
		}
		ev := f.next(t)
		got := ev.GetPollClosed().GetPoll()
		if ev.Audience != protobuf.RealtimeEvent_AUDIENCE_ALL || got.GetId() != 60 || got.GetOptions()[0].GetVotes() != 5 {
			t.Errorf("event = %v, want the final counts for everyone", ev)
		}
	})

	t.Run("none running is a 404", func(t *testing.T) {
		f := newChatWriteFixture(t, chatWriteOpts{courseChat: true})
		f.chat.EXPECT().GetActivePoll(uint(7)).Return(model.Poll{}, gorm.ErrRecordNotFound)

		_, err := f.api.CloseChatPoll(asCaller(chatAdmin), &protobuf.CloseChatPollRequest{StreamId: 7})

		wantChatCode(t, err, codes.NotFound)
		f.none(t)
	})
}

func TestListChatMessagesReportsWhatTheCallerMayDo(t *testing.T) {
	list := func(f chatWriteFixture, user *model.User) *protobuf.ChatAccess {
		var uid uint
		if user != nil {
			uid = user.ID
		}
		f.chat.EXPECT().GetVisibleChats(uid, uint(7)).Return(nil, nil).AnyTimes()
		f.chat.EXPECT().GetAllChats(uid, uint(7)).Return(nil, nil).AnyTimes()
		resp, err := f.api.ListChatMessages(asCaller(user), &protobuf.ListChatMessagesRequest{StreamId: 7})
		if err != nil {
			t.Fatalf("ListChatMessages: %v", err)
		}
		return resp.Access
	}

	t.Run("a student in a moderated chat with anonymous messages", func(t *testing.T) {
		got := list(newChatWriteFixture(t, chatWriteOpts{courseChat: true, moderated: true}), chatStudent)
		want := &protobuf.ChatAccess{CanWrite: true, AnonymousAllowed: true, Moderated: true}
		if got.CanWrite != want.CanWrite || got.CanModerate || !got.AnonymousAllowed || !got.Moderated {
			t.Errorf("access = %v, want %v", got, want)
		}
	})

	t.Run("an administrator moderates", func(t *testing.T) {
		if got := list(newChatWriteFixture(t, chatWriteOpts{courseChat: true, noAnonymous: true}), chatAdmin); !got.CanModerate || got.AnonymousAllowed {
			t.Errorf("access = %v", got)
		}
	})

	t.Run("an anonymous viewer, or a chat that is off, cannot write", func(t *testing.T) {
		if got := list(newChatWriteFixture(t, chatWriteOpts{courseChat: true, visibility: "public"}), nil); got.CanWrite || got.CanModerate {
			t.Errorf("anonymous access = %v", got)
		}
		if got := list(newChatWriteFixture(t, chatWriteOpts{}), chatStudent); got.CanWrite {
			t.Errorf("access with the chat off = %v", got)
		}
		if got := list(newChatWriteFixture(t, chatWriteOpts{streamChat: true}), chatStudent); !got.CanWrite {
			t.Errorf("access with only the stream's chat on = %v", got)
		}
	})
}
