package apiv2

import (
	"database/sql"
	"log/slog"
	"strings"
	"testing"
	"time"

	"go.uber.org/mock/gomock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"

	protobuf "github.com/TUM-Dev/gocast/apiv2/protobuf/server"
	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/mock_dao"
	"github.com/TUM-Dev/gocast/model"
	"github.com/TUM-Dev/gocast/pkg/realtimehub"
)

// chatWriteFixture is stream 7 of course 5 with a real hub, whose events the test reads.
type chatWriteFixture struct {
	api    *API
	chat   *mock_dao.MockChatDao
	events <-chan *protobuf.RealtimeEvent
}

type chatWriteOpts struct {
	visibility  string
	courseChat  bool
	streamChat  bool
	noAnonymous bool
	moderated   bool
}

func newChatWriteFixture(t *testing.T, o chatWriteOpts) chatWriteFixture {
	t.Helper()
	ctrl := gomock.NewController(t)
	if o.visibility == "" {
		o.visibility = "loggedin"
	}

	streams := mock_dao.NewMockStreamsDao(ctrl)
	streams.EXPECT().GetStreamByID(gomock.Any(), "7").
		Return(model.Stream{Model: gorm.Model{ID: 7}, CourseID: 5, ChatEnabled: o.streamChat}, nil).AnyTimes()
	courses := mock_dao.NewMockCoursesDao(ctrl)
	courses.EXPECT().GetCourseById(gomock.Any(), uint(5)).Return(model.Course{
		Model:                gorm.Model{ID: 5},
		Visibility:           o.visibility,
		ChatEnabled:          o.courseChat,
		AnonymousChatEnabled: !o.noAnonymous,
		ModeratedChatEnabled: o.moderated,
	}, nil).AnyTimes()
	chat := mock_dao.NewMockChatDao(ctrl)

	hub := realtimehub.NewMemory(realtimehub.WithViewersInterval(time.Hour))
	events, cancel := hub.Subscribe(7)
	t.Cleanup(cancel)

	return chatWriteFixture{
		api: &API{
			dao: dao.DaoWrapper{StreamsDao: streams, CoursesDao: courses, ChatDao: chat},
			log: slog.Default(),
			hub: hub,
		},
		chat:   chat,
		events: events,
	}
}

// next returns the next event that is not the hub's own viewer count.
func (f chatWriteFixture) next(t *testing.T) *protobuf.RealtimeEvent {
	t.Helper()
	for {
		select {
		case ev := <-f.events:
			if ev.GetViewers() == nil {
				return ev
			}
		case <-time.After(time.Second):
			t.Fatal("no event was published")
			return nil
		}
	}
}

// none asserts nothing was published.
func (f chatWriteFixture) none(t *testing.T) {
	t.Helper()
	for {
		select {
		case ev := <-f.events:
			if ev.GetViewers() == nil {
				t.Fatalf("published %v, want nothing", ev)
			}
		case <-time.After(50 * time.Millisecond):
			return
		}
	}
}

func wantChatCode(t *testing.T, err error, want codes.Code) {
	t.Helper()
	if got := status.Code(err); got != want {
		t.Fatalf("code = %v (%v), want %v", got, err, want)
	}
}

// storeMessage is AddMessage as the database does it: an id and a time, and the
// model's own refusals.
func storeMessage(id uint) func(*model.Chat) error {
	return func(c *model.Chat) error {
		c.ID = id
		c.CreatedAt = time.Date(2026, 10, 11, 10, 0, 0, 0, time.UTC)
		return nil
	}
}

func TestChatWritesArePolicedAsSignedInOnly(t *testing.T) {
	for _, name := range []string{"sendChatMessage", "toggleChatReaction", "votePoll"} {
		p, ok := methodPolicies["/protobuf.ChatService/"+name]
		if !ok {
			t.Fatalf("%s has no policy", name)
		}
		if p != authenticated {
			t.Errorf("%s policy = %+v, want authenticated: anonymous viewers read, they do not write", name, p)
		}
	}
}

func TestSendChatMessage(t *testing.T) {
	send := func(f chatWriteFixture, user *model.User, req *protobuf.SendChatMessageRequest) (*protobuf.SendChatMessageResponse, error) {
		req.StreamId = 7
		return f.api.SendChatMessage(asCaller(user), req)
	}

	t.Run("stores the message and tells the stream's viewers", func(t *testing.T) {
		f := newChatWriteFixture(t, chatWriteOpts{courseChat: true})
		f.chat.EXPECT().AddMessage(gomock.Any()).DoAndReturn(func(c *model.Chat) error {
			if c.UserID != "3" || c.UserName != "Studi" || c.StreamID != 7 || c.Admin || !c.Visible.Bool {
				t.Errorf("stored %+v", c)
			}
			return storeMessage(40)(c)
		})

		resp, err := send(f, chatStudent, &protobuf.SendChatMessageRequest{Text: "hi <b>there</b> https://tum.de"})
		if err != nil {
			t.Fatalf("SendChatMessage: %v", err)
		}
		if resp.Message.Id != 40 || resp.Message.UserId != 3 || resp.Message.MessageText != "hi <b>there</b> https://tum.de" {
			t.Errorf("response = %v", resp.Message)
		}
		// The html field is what a read returns: escaped, with the link anchored.
		if !strings.Contains(resp.Message.MessageHtml, "&lt;b&gt;") || !strings.Contains(resp.Message.MessageHtml, `<a href="https://tum.de"`) {
			t.Errorf("html = %q", resp.Message.MessageHtml)
		}

		ev := f.next(t)
		if ev.Audience != protobuf.RealtimeEvent_AUDIENCE_ALL || ev.StreamId != 7 {
			t.Errorf("event = %v, want everyone, stream 7", ev)
		}
		if ev.GetMessage().GetMessage().GetId() != 40 || ev.GetMessage().GetMessage().GetMessageHtml() != resp.Message.MessageHtml {
			t.Errorf("event message = %v", ev.GetMessage())
		}
	})

	t.Run("chat counts as on when only the stream has it on", func(t *testing.T) {
		f := newChatWriteFixture(t, chatWriteOpts{streamChat: true})
		f.chat.EXPECT().AddMessage(gomock.Any()).DoAndReturn(storeMessage(1))

		if _, err := send(f, chatStudent, &protobuf.SendChatMessageRequest{Text: "hi"}); err != nil {
			t.Fatalf("SendChatMessage: %v", err)
		}
	})

	t.Run("refuses when the chat is off", func(t *testing.T) {
		f := newChatWriteFixture(t, chatWriteOpts{})

		_, err := send(f, chatStudent, &protobuf.SendChatMessageRequest{Text: "hi"})

		wantChatCode(t, err, codes.PermissionDenied)
		f.none(t)
	})

	t.Run("refuses an anonymous caller", func(t *testing.T) {
		// A public course, so the caller is admitted to read and only the write refuses.
		f := newChatWriteFixture(t, chatWriteOpts{courseChat: true, visibility: "public"})

		_, err := send(f, nil, &protobuf.SendChatMessageRequest{Text: "hi"})

		wantChatCode(t, err, codes.Unauthenticated)
		f.none(t)
	})

	t.Run("refuses a user the course does not admit", func(t *testing.T) {
		f := newChatWriteFixture(t, chatWriteOpts{courseChat: true, visibility: "enrolled"})

		_, err := send(f, chatStudent, &protobuf.SendChatMessageRequest{Text: "hi"})

		wantChatCode(t, err, codes.PermissionDenied)
		f.none(t)
	})

	t.Run("refuses a stream that does not exist", func(t *testing.T) {
		f := newChatWriteFixture(t, chatWriteOpts{courseChat: true})
		f.api.dao.StreamsDao.(*mock_dao.MockStreamsDao).EXPECT().GetStreamByID(gomock.Any(), "99").
			Return(model.Stream{}, gorm.ErrRecordNotFound)

		_, err := f.api.SendChatMessage(asCaller(chatStudent), &protobuf.SendChatMessageRequest{StreamId: 99, Text: "hi"})

		wantChatCode(t, err, codes.NotFound)
	})

	t.Run("a cooled down user gets a 429 and nothing is published", func(t *testing.T) {
		f := newChatWriteFixture(t, chatWriteOpts{courseChat: true})
		f.chat.EXPECT().AddMessage(gomock.Any()).Return(model.ErrCooledDown)

		_, err := send(f, chatStudent, &protobuf.SendChatMessageRequest{Text: "hi"})

		wantChatCode(t, err, codes.ResourceExhausted)
		f.none(t)
	})

	t.Run("an administrator is stored as one, which is what exempts them from the cooldown", func(t *testing.T) {
		// The cooldown lives in the model's BeforeCreate and skips Admin messages;
		// what this handler owes it is the flag.
		f := newChatWriteFixture(t, chatWriteOpts{courseChat: true, moderated: true})
		f.chat.EXPECT().AddMessage(gomock.Any()).DoAndReturn(func(c *model.Chat) error {
			if !c.Admin || !c.Visible.Bool {
				t.Errorf("stored %+v, want an admin's visible message even in a moderated chat", c)
			}
			return storeMessage(2)(c)
		})

		if _, err := send(f, chatAdmin, &protobuf.SendChatMessageRequest{Text: "hi"}); err != nil {
			t.Fatalf("SendChatMessage: %v", err)
		}
		if ev := f.next(t); ev.Audience != protobuf.RealtimeEvent_AUDIENCE_ALL {
			t.Errorf("audience = %v, want everyone", ev.Audience)
		}
	})

	t.Run("a moderated chat holds an ordinary message for its author and the administrators", func(t *testing.T) {
		f := newChatWriteFixture(t, chatWriteOpts{courseChat: true, moderated: true})
		f.chat.EXPECT().AddMessage(gomock.Any()).DoAndReturn(func(c *model.Chat) error {
			if c.Visible.Bool {
				t.Error("stored visible")
			}
			return storeMessage(3)(c)
		})

		resp, err := send(f, chatStudent, &protobuf.SendChatMessageRequest{Text: "hi"})
		if err != nil {
			t.Fatalf("SendChatMessage: %v", err)
		}
		if resp.Message.Visible {
			t.Error("response says visible")
		}

		ev := f.next(t)
		if ev.Audience != protobuf.RealtimeEvent_AUDIENCE_COURSE_ADMINS || ev.AudienceUserId != 3 {
			t.Errorf("audience = %v for user %d, want course admins and the author, 3", ev.Audience, ev.AudienceUserId)
		}
		if ev.GetMessage().GetMessage().GetVisible() {
			t.Error("event says visible")
		}
	})

	t.Run("an anonymous message hides the author from the event but not from the author", func(t *testing.T) {
		f := newChatWriteFixture(t, chatWriteOpts{courseChat: true})
		f.chat.EXPECT().AddMessage(gomock.Any()).DoAndReturn(func(c *model.Chat) error {
			if c.UserName != "Anonymous" || c.UserID != "3" {
				t.Errorf("stored %+v", c)
			}
			return storeMessage(4)(c)
		})

		resp, err := send(f, chatStudent, &protobuf.SendChatMessageRequest{Text: "hi", Anonymous: true})
		if err != nil {
			t.Fatalf("SendChatMessage: %v", err)
		}
		if resp.Message.UserId != 3 || resp.Message.UserName != "Anonymous" {
			t.Errorf("response = %v", resp.Message)
		}
		if m := f.next(t).GetMessage().GetMessage(); m.UserId != 0 || m.UserName != "Anonymous" {
			t.Errorf("event message = %v: it names the author", m)
		}
	})

	t.Run("refuses an anonymous message where the course does not allow them", func(t *testing.T) {
		f := newChatWriteFixture(t, chatWriteOpts{courseChat: true, noAnonymous: true})

		_, err := send(f, chatStudent, &protobuf.SendChatMessageRequest{Text: "hi", Anonymous: true})

		wantChatCode(t, err, codes.InvalidArgument)
		f.none(t)
	})

	for name, tc := range map[string]struct {
		err  error
		want codes.Code
	}{
		"too long":                             {model.ErrMessageTooLong, codes.InvalidArgument},
		"empty":                                {model.ErrMessageNoText, codes.InvalidArgument},
		"reply to a reply":                     {model.ErrReplyToReply, codes.InvalidArgument},
		"reply to a message of another stream": {model.ErrReplyToWrongStream, codes.InvalidArgument},
		"reply to nothing":                     {model.ErrReplyToNoMsg, codes.NotFound},
	} {
		t.Run("the model's refusal, "+name, func(t *testing.T) {
			f := newChatWriteFixture(t, chatWriteOpts{courseChat: true})
			f.chat.EXPECT().AddMessage(gomock.Any()).Return(tc.err)

			_, err := send(f, chatStudent, &protobuf.SendChatMessageRequest{Text: "hi"})

			wantChatCode(t, err, tc.want)
			f.none(t)
		})
	}

	t.Run("a reply to a message of another stream is refused", func(t *testing.T) {
		f := newChatWriteFixture(t, chatWriteOpts{courseChat: true})
		// The DAO's own check, in BeforeCreate, is what knows the parent's stream.
		f.chat.EXPECT().GetChat(uint(30), uint(3)).Return(&model.Chat{
			Model: gorm.Model{ID: 30}, StreamID: 8, Visible: sql.NullBool{Bool: true, Valid: true},
		}, nil)
		f.chat.EXPECT().AddMessage(gomock.Any()).DoAndReturn(func(c *model.Chat) error {
			if !c.ReplyTo.Valid || c.ReplyTo.Int64 != 30 {
				t.Errorf("reply_to = %v", c.ReplyTo)
			}
			return model.ErrReplyToWrongStream
		})
		reply := uint32(30)

		_, err := send(f, chatStudent, &protobuf.SendChatMessageRequest{Text: "hi", ReplyTo: &reply})

		wantChatCode(t, err, codes.InvalidArgument)
		f.none(t)
	})

	t.Run("a reply carries its parent and is published as one", func(t *testing.T) {
		f := newChatWriteFixture(t, chatWriteOpts{courseChat: true})
		f.chat.EXPECT().GetChat(uint(30), uint(3)).Return(&model.Chat{
			Model: gorm.Model{ID: 30}, StreamID: 7, Visible: sql.NullBool{Bool: true, Valid: true},
		}, nil)
		f.chat.EXPECT().AddMessage(gomock.Any()).DoAndReturn(storeMessage(31))
		reply := uint32(30)

		resp, err := send(f, chatStudent, &protobuf.SendChatMessageRequest{Text: "hi", ReplyTo: &reply})
		if err != nil {
			t.Fatalf("SendChatMessage: %v", err)
		}
		if resp.Message.ReplyTo != 30 || f.next(t).GetMessage().GetMessage().GetReplyTo() != 30 {
			t.Error("the reply lost its parent")
		}
	})

	t.Run("a reply to a message that does not exist or is hidden from the caller is a 404", func(t *testing.T) {
		for name, parent := range map[string]*model.Chat{
			"missing": {},
			"awaiting approval, by someone else": {
				Model: gorm.Model{ID: 30}, StreamID: 7, UserID: "8", Visible: sql.NullBool{Valid: true},
			},
		} {
			t.Run(name, func(t *testing.T) {
				f := newChatWriteFixture(t, chatWriteOpts{courseChat: true})
				f.chat.EXPECT().GetChat(uint(30), uint(3)).Return(parent, nil)
				reply := uint32(30)

				_, err := send(f, chatStudent, &protobuf.SendChatMessageRequest{Text: "hi", ReplyTo: &reply})

				wantChatCode(t, err, codes.NotFound)
			})
		}
	})

	t.Run("an administrator may reply to a message awaiting approval", func(t *testing.T) {
		f := newChatWriteFixture(t, chatWriteOpts{courseChat: true, moderated: true})
		f.chat.EXPECT().GetChat(uint(30), uint(9)).Return(&model.Chat{
			Model: gorm.Model{ID: 30}, StreamID: 7, UserID: "3", Visible: sql.NullBool{Valid: true},
		}, nil)
		f.chat.EXPECT().AddMessage(gomock.Any()).DoAndReturn(storeMessage(32))
		reply := uint32(30)

		if _, err := send(f, chatAdmin, &protobuf.SendChatMessageRequest{Text: "ok", ReplyTo: &reply}); err != nil {
			t.Fatalf("SendChatMessage: %v", err)
		}
	})

	t.Run("mentions are deduplicated and bounded", func(t *testing.T) {
		f := newChatWriteFixture(t, chatWriteOpts{courseChat: true})
		f.chat.EXPECT().AddMessage(gomock.Any()).DoAndReturn(func(c *model.Chat) error {
			if len(c.AddressedToIds) != 2 || c.AddressedToIds[0] != 9 || c.AddressedToIds[1] != 4 {
				t.Errorf("addressed to %v, want [9 4]", c.AddressedToIds)
			}
			return storeMessage(5)(c)
		})
		if _, err := send(f, chatStudent, &protobuf.SendChatMessageRequest{Text: "hi", AddressedTo: []uint32{9, 0, 9, 4}}); err != nil {
			t.Fatalf("SendChatMessage: %v", err)
		}

		_, err := send(f, chatStudent, &protobuf.SendChatMessageRequest{Text: "hi", AddressedTo: make([]uint32, 21)})
		wantChatCode(t, err, codes.InvalidArgument)
	})
}

func TestToggleChatReaction(t *testing.T) {
	visible := sql.NullBool{Bool: true, Valid: true}
	toggle := func(f chatWriteFixture, user *model.User, emoji string) (*protobuf.ToggleChatReactionResponse, error) {
		return f.api.ToggleChatReaction(asCaller(user), &protobuf.ToggleChatReactionRequest{StreamId: 7, MessageId: 30, Emoji: emoji})
	}

	t.Run("adds a reaction and pushes the message's reactions", func(t *testing.T) {
		f := newChatWriteFixture(t, chatWriteOpts{courseChat: true})
		f.chat.EXPECT().GetChat(uint(30), uint(3)).Return(&model.Chat{Model: gorm.Model{ID: 30}, StreamID: 7, Visible: visible}, nil)
		f.chat.EXPECT().ToggleReaction(uint(3), uint(30), "Studi", "tada").Return(nil)
		f.chat.EXPECT().GetReactions(uint(30)).Return([]model.ChatReaction{{ChatID: 30, UserID: 3, Username: "Studi", Emoji: "tada"}}, nil)

		resp, err := toggle(f, chatStudent, "tada")
		if err != nil {
			t.Fatalf("ToggleChatReaction: %v", err)
		}
		if len(resp.Reactions) != 1 || resp.Reactions[0].Emoji != "tada" || resp.Reactions[0].Count != 1 {
			t.Errorf("response = %v", resp.Reactions)
		}
		ev := f.next(t)
		got := ev.GetReactions()
		if ev.Audience != protobuf.RealtimeEvent_AUDIENCE_ALL || got.GetMessageId() != 30 || len(got.GetReactions()) != 1 {
			t.Errorf("event = %v", ev)
		}
	})

	t.Run("toggling off publishes the empty set", func(t *testing.T) {
		f := newChatWriteFixture(t, chatWriteOpts{courseChat: true})
		f.chat.EXPECT().GetChat(uint(30), uint(3)).Return(&model.Chat{Model: gorm.Model{ID: 30}, StreamID: 7, Visible: visible}, nil)
		f.chat.EXPECT().ToggleReaction(uint(3), uint(30), "Studi", "tada").Return(nil)
		f.chat.EXPECT().GetReactions(uint(30)).Return(nil, nil)

		resp, err := toggle(f, chatStudent, "tada")
		if err != nil {
			t.Fatalf("ToggleChatReaction: %v", err)
		}
		if len(resp.Reactions) != 0 {
			t.Errorf("response = %v", resp.Reactions)
		}
		if ev := f.next(t); ev.GetReactions() == nil || len(ev.GetReactions().Reactions) != 0 {
			t.Errorf("event = %v, want a reactions event with no reactions", ev)
		}
	})

	t.Run("refuses an emoji that is not offered", func(t *testing.T) {
		f := newChatWriteFixture(t, chatWriteOpts{courseChat: true})

		_, err := toggle(f, chatStudent, "skull")

		wantChatCode(t, err, codes.InvalidArgument)
		f.none(t)
	})

	t.Run("a message of another stream, or none, is a 404", func(t *testing.T) {
		for name, chat := range map[string]*model.Chat{
			"another stream": {Model: gorm.Model{ID: 30}, StreamID: 8, Visible: visible},
			"missing":        {},
		} {
			t.Run(name, func(t *testing.T) {
				f := newChatWriteFixture(t, chatWriteOpts{courseChat: true})
				f.chat.EXPECT().GetChat(uint(30), uint(3)).Return(chat, nil)

				_, err := toggle(f, chatStudent, "tada")

				wantChatCode(t, err, codes.NotFound)
				f.none(t)
			})
		}
	})

	t.Run("a message awaiting approval is a 404 to a student and reactable by an administrator, privately", func(t *testing.T) {
		hidden := func() *model.Chat {
			return &model.Chat{Model: gorm.Model{ID: 30}, StreamID: 7, UserID: "8", Visible: sql.NullBool{Valid: true}}
		}
		f := newChatWriteFixture(t, chatWriteOpts{courseChat: true, moderated: true})
		f.chat.EXPECT().GetChat(uint(30), uint(3)).Return(hidden(), nil)
		_, err := toggle(f, chatStudent, "tada")
		wantChatCode(t, err, codes.NotFound)

		f.chat.EXPECT().GetChat(uint(30), uint(9)).Return(hidden(), nil)
		f.chat.EXPECT().ToggleReaction(uint(9), uint(30), "Dozent", "tada").Return(nil)
		f.chat.EXPECT().GetReactions(uint(30)).Return(nil, nil)
		if _, err := toggle(f, chatAdmin, "tada"); err != nil {
			t.Fatalf("ToggleChatReaction: %v", err)
		}
		if ev := f.next(t); ev.Audience != protobuf.RealtimeEvent_AUDIENCE_COURSE_ADMINS {
			t.Errorf("audience = %v, want course admins", ev.Audience)
		}
	})

	t.Run("refuses an anonymous caller and a closed chat", func(t *testing.T) {
		f := newChatWriteFixture(t, chatWriteOpts{courseChat: true, visibility: "public"})
		_, err := toggle(f, nil, "tada")
		wantChatCode(t, err, codes.Unauthenticated)

		off := newChatWriteFixture(t, chatWriteOpts{})
		_, err = toggle(off, chatStudent, "tada")
		wantChatCode(t, err, codes.PermissionDenied)
	})

	t.Run("refuses a user the course does not admit", func(t *testing.T) {
		f := newChatWriteFixture(t, chatWriteOpts{courseChat: true, visibility: "enrolled"})

		_, err := toggle(f, chatStudent, "tada")

		wantChatCode(t, err, codes.PermissionDenied)
	})
}

func TestVotePoll(t *testing.T) {
	poll := model.Poll{
		Model:    gorm.Model{ID: 60},
		StreamID: 7,
		Question: "Q?",
		Active:   true,
		PollOptions: []model.PollOption{
			{Model: gorm.Model{ID: 601}, Answer: "yes"},
			{Model: gorm.Model{ID: 602}, Answer: "no"},
		},
	}
	vote := func(f chatWriteFixture, user *model.User, option uint32) (*protobuf.VoteChatPollResponse, error) {
		return f.api.VotePoll(asCaller(user), &protobuf.VoteChatPollRequest{StreamId: 7, PollOptionId: option})
	}

	t.Run("records the vote and tells the administrators the tally", func(t *testing.T) {
		f := newChatWriteFixture(t, chatWriteOpts{courseChat: true})
		f.chat.EXPECT().GetActivePoll(uint(7)).Return(poll, nil)
		f.chat.EXPECT().GetPollUserVote(uint(60), uint(3)).Return(uint(0), nil)
		f.chat.EXPECT().AddChatPollOptionVote(uint(601), uint(3)).Return(nil)
		f.chat.EXPECT().GetPollOptionVoteCount(uint(601)).Return(int64(4), nil)

		resp, err := vote(f, chatStudent, 601)
		if err != nil {
			t.Fatalf("VotePoll: %v", err)
		}
		if resp.Poll.VotedOptionId != 601 {
			t.Errorf("voted option = %d", resp.Poll.VotedOptionId)
		}
		// A student must not learn the tally while the poll runs.
		for _, o := range resp.Poll.Options {
			if o.Votes != 0 {
				t.Errorf("option %d shows %d votes to a student", o.Id, o.Votes)
			}
		}
		ev := f.next(t)
		if ev.Audience != protobuf.RealtimeEvent_AUDIENCE_COURSE_ADMINS ||
			ev.GetPollVote().GetPollOptionId() != 601 || ev.GetPollVote().GetVotes() != 4 {
			t.Errorf("event = %v", ev)
		}
	})

	t.Run("a second vote is a conflict", func(t *testing.T) {
		f := newChatWriteFixture(t, chatWriteOpts{courseChat: true})
		f.chat.EXPECT().GetActivePoll(uint(7)).Return(poll, nil)
		f.chat.EXPECT().GetPollUserVote(uint(60), uint(3)).Return(uint(602), nil)

		_, err := vote(f, chatStudent, 601)

		wantChatCode(t, err, codes.AlreadyExists)
		f.none(t)
	})

	t.Run("a poll that closed, or none running, is a 404", func(t *testing.T) {
		f := newChatWriteFixture(t, chatWriteOpts{courseChat: true})
		f.chat.EXPECT().GetActivePoll(uint(7)).Return(model.Poll{}, gorm.ErrRecordNotFound)

		_, err := vote(f, chatStudent, 601)

		wantChatCode(t, err, codes.NotFound)
		f.none(t)
	})

	t.Run("an option of another poll is a 404", func(t *testing.T) {
		f := newChatWriteFixture(t, chatWriteOpts{courseChat: true})
		f.chat.EXPECT().GetActivePoll(uint(7)).Return(poll, nil)

		_, err := vote(f, chatStudent, 999)

		wantChatCode(t, err, codes.NotFound)
		f.none(t)
	})

	t.Run("an administrator's response carries the counts", func(t *testing.T) {
		f := newChatWriteFixture(t, chatWriteOpts{courseChat: true})
		f.chat.EXPECT().GetActivePoll(uint(7)).Return(poll, nil)
		f.chat.EXPECT().GetPollUserVote(uint(60), uint(9)).Return(uint(0), nil)
		f.chat.EXPECT().AddChatPollOptionVote(uint(602), uint(9)).Return(nil)
		f.chat.EXPECT().GetPollOptionVoteCount(uint(602)).Return(int64(1), nil).Times(2)
		f.chat.EXPECT().GetPollOptionVoteCount(uint(601)).Return(int64(0), nil)

		resp, err := vote(f, chatAdmin, 602)
		if err != nil {
			t.Fatalf("VotePoll: %v", err)
		}
		if resp.Poll.Options[1].Votes != 1 {
			t.Errorf("options = %v", resp.Poll.Options)
		}
	})

	t.Run("refuses an anonymous caller, a closed chat and a stranger", func(t *testing.T) {
		f := newChatWriteFixture(t, chatWriteOpts{courseChat: true, visibility: "public"})
		_, err := vote(f, nil, 601)
		wantChatCode(t, err, codes.Unauthenticated)

		off := newChatWriteFixture(t, chatWriteOpts{})
		_, err = vote(off, chatStudent, 601)
		wantChatCode(t, err, codes.PermissionDenied)

		enrolled := newChatWriteFixture(t, chatWriteOpts{courseChat: true, visibility: "enrolled"})
		_, err = vote(enrolled, chatStudent, 601)
		wantChatCode(t, err, codes.PermissionDenied)
	})
}
