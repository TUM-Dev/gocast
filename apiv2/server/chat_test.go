package apiv2

import (
	"database/sql"
	"log/slog"
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
)

// chatFixture is stream 7 of course 5, with the course's visibility up to the test.
type chatFixture struct {
	api  *API
	chat *mock_dao.MockChatDao
}

func newChatFixture(t *testing.T, visibility string) chatFixture {
	t.Helper()
	ctrl := gomock.NewController(t)

	streams := mock_dao.NewMockStreamsDao(ctrl)
	streams.EXPECT().GetStreamByID(gomock.Any(), "7").
		Return(model.Stream{Model: gorm.Model{ID: 7}, CourseID: 5}, nil).AnyTimes()
	courses := mock_dao.NewMockCoursesDao(ctrl)
	courses.EXPECT().GetCourseById(gomock.Any(), uint(5)).
		Return(model.Course{Model: gorm.Model{ID: 5}, Visibility: visibility}, nil).AnyTimes()
	chat := mock_dao.NewMockChatDao(ctrl)

	return chatFixture{
		api: &API{
			dao: dao.DaoWrapper{StreamsDao: streams, CoursesDao: courses, ChatDao: chat},
			log: slog.Default(),
		},
		chat: chat,
	}
}

var (
	chatStudent = &model.User{Model: gorm.Model{ID: 3}, Name: "Studi"}
	chatAdmin   = &model.User{
		Model:               gorm.Model{ID: 9},
		Name:                "Dozent",
		AdministeredCourses: []model.Course{{Model: gorm.Model{ID: 5}}},
	}
)

func TestListChatMessages(t *testing.T) {
	created := time.Date(2026, 10, 5, 10, 0, 0, 0, time.UTC)
	chats := func() []model.Chat {
		return []model.Chat{
			{
				Model:            gorm.Model{ID: 1, CreatedAt: created},
				UserID:           "3",
				UserName:         "Anonymous",
				Message:          "see https://tum.de",
				SanitizedMessage: `see <a href="https://tum.de">https://tum.de</a>`,
				Color:            "#e64f7a",
				Visible:          sql.NullBool{Bool: true, Valid: true},
				AddressedToIds:   []uint{9},
				Reactions: []model.ChatReaction{
					{ChatID: 1, UserID: 4, Username: "B", Emoji: "tada"},
					{ChatID: 1, UserID: 5, Username: "C", Emoji: "+1"},
					{ChatID: 1, UserID: 6, Username: "D", Emoji: "tada"},
				},
				Replies: []model.Chat{{
					Model:    gorm.Model{ID: 2, CreatedAt: created},
					UserID:   "9",
					UserName: "Dozent",
					Message:  "yes",
					Admin:    true,
					Visible:  sql.NullBool{Bool: true, Valid: true},
					ReplyTo:  sql.NullInt64{Int64: 1, Valid: true},
				}},
			},
		}
	}

	t.Run("an anonymous viewer of a public course gets the visible messages", func(t *testing.T) {
		f := newChatFixture(t, "public")
		f.chat.EXPECT().GetVisibleChats(uint(0), uint(7)).Return(chats(), nil)

		resp, err := f.api.ListChatMessages(asCaller(nil), &protobuf.ListChatMessagesRequest{StreamId: 7})
		if err != nil {
			t.Fatalf("ListChatMessages: %v", err)
		}

		if len(resp.Messages) != 1 {
			t.Fatalf("got %d messages, want 1", len(resp.Messages))
		}
		msg := resp.Messages[0]
		if msg.UserId != 0 {
			t.Errorf("anonymous message carries its author's id %d", msg.UserId)
		}
		if msg.MessageText != "see https://tum.de" || msg.MessageHtml == msg.MessageText {
			t.Errorf("text = %q, html = %q", msg.MessageText, msg.MessageHtml)
		}
		if !msg.CreatedAt.AsTime().Equal(created) || !msg.Visible || msg.Color != "#e64f7a" {
			t.Errorf("message = %v", msg)
		}
		if len(msg.AddressedTo) != 1 || msg.AddressedTo[0] != 9 {
			t.Errorf("addressed to %v, want [9]", msg.AddressedTo)
		}

		// Grouped by emoji, in the picker's order rather than the reactions'.
		if len(msg.Reactions) != 2 || msg.Reactions[0].Emoji != "+1" || msg.Reactions[1].Emoji != "tada" {
			t.Fatalf("reactions = %v", msg.Reactions)
		}
		if tada := msg.Reactions[1]; tada.Count != 2 || len(tada.UserIds) != 2 || tada.UserNames[1] != "D" {
			t.Errorf("tada = %v", tada)
		}

		if len(msg.Replies) != 1 || msg.Replies[0].ReplyTo != 1 || msg.Replies[0].UserId != 9 || !msg.Replies[0].Admin {
			t.Errorf("replies = %v", msg.Replies)
		}
	})

	t.Run("the author of an anonymous message still sees it as theirs", func(t *testing.T) {
		f := newChatFixture(t, "public")
		f.chat.EXPECT().GetVisibleChats(uint(3), uint(7)).Return(chats(), nil)

		resp, err := f.api.ListChatMessages(asCaller(chatStudent), &protobuf.ListChatMessagesRequest{StreamId: 7})
		if err != nil {
			t.Fatalf("ListChatMessages: %v", err)
		}
		if got := resp.Messages[0].UserId; got != 3 {
			t.Errorf("user id = %d, want the caller's own 3", got)
		}
	})

	t.Run("a course administrator also gets the messages awaiting approval", func(t *testing.T) {
		f := newChatFixture(t, "enrolled")
		f.chat.EXPECT().GetAllChats(uint(9), uint(7)).Return(nil, nil)

		resp, err := f.api.ListChatMessages(asCaller(chatAdmin), &protobuf.ListChatMessagesRequest{StreamId: 7})
		if err != nil {
			t.Fatalf("ListChatMessages: %v", err)
		}
		if len(resp.Messages) != 0 {
			t.Errorf("messages = %v", resp.Messages)
		}
	})

	t.Run("refuses an anonymous viewer of a course for signed-in users", func(t *testing.T) {
		f := newChatFixture(t, "loggedin")

		_, err := f.api.ListChatMessages(asCaller(nil), &protobuf.ListChatMessagesRequest{StreamId: 7})
		if status.Code(err) != codes.PermissionDenied {
			t.Fatalf("code = %v, want PermissionDenied", status.Code(err))
		}
	})

	t.Run("refuses a student not enrolled in an enrolled-only course", func(t *testing.T) {
		f := newChatFixture(t, "enrolled")

		_, err := f.api.ListChatMessages(asCaller(chatStudent), &protobuf.ListChatMessagesRequest{StreamId: 7})
		if status.Code(err) != codes.PermissionDenied {
			t.Fatalf("code = %v, want PermissionDenied", status.Code(err))
		}
	})
}

func TestGetActivePoll(t *testing.T) {
	poll := model.Poll{
		Model:    gorm.Model{ID: 11},
		Question: "Weißbier?",
		Active:   true,
		PollOptions: []model.PollOption{
			{Model: gorm.Model{ID: 21}, Answer: "Ja"},
			{Model: gorm.Model{ID: 22}, Answer: "Nein"},
		},
	}
	req := &protobuf.GetActiveChatPollRequest{StreamId: 7}

	t.Run("answers empty when no poll is running", func(t *testing.T) {
		f := newChatFixture(t, "public")
		f.chat.EXPECT().GetActivePoll(uint(7)).Return(model.Poll{}, gorm.ErrRecordNotFound)

		resp, err := f.api.GetActivePoll(asCaller(nil), req)
		if err != nil {
			t.Fatalf("GetActivePoll: %v", err)
		}
		if resp.Poll != nil {
			t.Errorf("poll = %v, want none", resp.Poll)
		}
	})

	t.Run("a student sees their vote but no counts", func(t *testing.T) {
		f := newChatFixture(t, "public")
		f.chat.EXPECT().GetActivePoll(uint(7)).Return(poll, nil)
		f.chat.EXPECT().GetPollUserVote(uint(11), uint(3)).Return(uint(22), nil)

		resp, err := f.api.GetActivePoll(asCaller(chatStudent), req)
		if err != nil {
			t.Fatalf("GetActivePoll: %v", err)
		}
		if resp.Poll.VotedOptionId != 22 || resp.Poll.Question != "Weißbier?" || len(resp.Poll.Options) != 2 {
			t.Errorf("poll = %v", resp.Poll)
		}
		for _, o := range resp.Poll.Options {
			if o.Votes != 0 {
				t.Errorf("option %d shows %d votes to a student", o.Id, o.Votes)
			}
		}
	})

	t.Run("an anonymous viewer sees the poll without asking for a vote", func(t *testing.T) {
		f := newChatFixture(t, "public")
		f.chat.EXPECT().GetActivePoll(uint(7)).Return(poll, nil)

		resp, err := f.api.GetActivePoll(asCaller(nil), req)
		if err != nil {
			t.Fatalf("GetActivePoll: %v", err)
		}
		if resp.Poll.VotedOptionId != 0 {
			t.Errorf("voted = %d", resp.Poll.VotedOptionId)
		}
	})

	t.Run("an administrator sees the counts", func(t *testing.T) {
		f := newChatFixture(t, "public")
		f.chat.EXPECT().GetActivePoll(uint(7)).Return(poll, nil)
		f.chat.EXPECT().GetPollUserVote(uint(11), uint(9)).Return(uint(0), nil)
		f.chat.EXPECT().GetPollOptionVoteCount(uint(21)).Return(int64(4), nil)
		f.chat.EXPECT().GetPollOptionVoteCount(uint(22)).Return(int64(1), nil)

		resp, err := f.api.GetActivePoll(asCaller(chatAdmin), req)
		if err != nil {
			t.Fatalf("GetActivePoll: %v", err)
		}
		if resp.Poll.Options[0].Votes != 4 || resp.Poll.Options[1].Votes != 1 {
			t.Errorf("options = %v", resp.Poll.Options)
		}
	})
}

func TestListPolls(t *testing.T) {
	req := &protobuf.ListChatPollsRequest{StreamId: 7}

	t.Run("is the course administrators'", func(t *testing.T) {
		f := newChatFixture(t, "public")

		_, err := f.api.ListPolls(asCaller(chatStudent), req)
		if status.Code(err) != codes.PermissionDenied {
			t.Fatalf("code = %v, want PermissionDenied", status.Code(err))
		}
	})

	t.Run("lists the closed polls with their results", func(t *testing.T) {
		f := newChatFixture(t, "public")
		f.chat.EXPECT().GetPolls(uint(7)).Return([]model.Poll{{
			Model:       gorm.Model{ID: 11},
			Question:    "Weißbier?",
			PollOptions: []model.PollOption{{Model: gorm.Model{ID: 21}, Answer: "Ja"}},
		}}, nil)
		f.chat.EXPECT().GetPollOptionVoteCount(uint(21)).Return(int64(7), nil)

		resp, err := f.api.ListPolls(asCaller(chatAdmin), req)
		if err != nil {
			t.Fatalf("ListPolls: %v", err)
		}
		if len(resp.Polls) != 1 || resp.Polls[0].Active || resp.Polls[0].Options[0].Votes != 7 {
			t.Errorf("polls = %v", resp.Polls)
		}
	})
}

func TestListChatUsers(t *testing.T) {
	f := newChatFixture(t, "public")
	f.chat.EXPECT().GetChatUsers(uint(7)).Return([]model.User{
		{Model: gorm.Model{ID: 3}, Name: "Studi"},
		{
			Model:    gorm.Model{ID: 4},
			Name:     "Maximilian",
			Settings: []model.UserSetting{{Type: model.PreferredName, Value: "Max"}},
		},
	}, nil)

	resp, err := f.api.ListChatUsers(asCaller(nil), &protobuf.ListChatUsersRequest{StreamId: 7})
	if err != nil {
		t.Fatalf("ListChatUsers: %v", err)
	}
	if len(resp.Users) != 2 || resp.Users[0].Name != "Studi" || resp.Users[1].Name != "Max" || resp.Users[1].Id != 4 {
		t.Errorf("users = %v", resp.Users)
	}
}
