package api

import (
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/golang/mock/gomock"
	"github.com/joschahenningsen/TUM-Live/dao"
	"github.com/joschahenningsen/TUM-Live/mock_dao"
	"github.com/joschahenningsen/TUM-Live/model"
	"github.com/joschahenningsen/TUM-Live/tools"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMessages(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Run("GET[no context]", func(t *testing.T) {
		streamId := uint(1234)

		w := httptest.NewRecorder()
		c, r := gin.CreateTestContext(w)

		chat := r.Group("/api/chat")
		configGinChatRouter(chat, dao.DaoWrapper{})
		c.Request, _ = http.NewRequest(http.MethodGet, fmt.Sprintf("/api/chat/%d/messages", streamId), nil)
		r.ServeHTTP(w, c.Request)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})
	t.Run("GET[success isAdmin=true]", func(t *testing.T) {
		uid := uint(1)
		streamId := uint(1234)
		courseId := uint(1111)
		chats := []model.Chat{
			{Message: "1", IsVisible: false},
			{Message: "2", IsVisible: true},
			{Message: "3", IsVisible: true},
		}

		w := httptest.NewRecorder()
		c, r := gin.CreateTestContext(w)

		r.Use(func(c *gin.Context) {
			c.Set("TUMLiveContext", tools.TUMLiveContext{User: &model.User{
				Model: gorm.Model{ID: uid},
				Name:  "Admin",
				Role:  model.AdminType,
			}})
		})

		// chat mock
		chatMock := mock_dao.NewMockChatDao(gomock.NewController(t))
		chatMock.EXPECT().GetAllChats(uid, streamId).Return(chats, nil).AnyTimes()

		// streams mock
		streamsMock := mock_dao.NewMockStreamsDao(gomock.NewController(t))
		streamsMock.EXPECT().GetStreamByID(gomock.Any(), fmt.Sprintf("%d", streamId)).Return(model.Stream{
			Model:    gorm.Model{ID: streamId},
			CourseID: courseId,
		}, nil).AnyTimes()

		// course mock
		courseMock := mock_dao.NewMockCoursesDao(gomock.NewController(t))
		courseMock.EXPECT().GetCourseById(gomock.Any(), courseId).Return(model.Course{}, nil).AnyTimes()

		chat := r.Group("/api/chat")
		configGinChatRouter(chat, dao.DaoWrapper{ChatDao: chatMock, StreamsDao: streamsMock, CoursesDao: courseMock})

		c.Request, _ = http.NewRequest(http.MethodGet, fmt.Sprintf("/api/chat/%d/messages", streamId), nil)
		r.ServeHTTP(w, c.Request)

		j, _ := json.Marshal(chats)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, string(j), w.Body.String())
	})
	t.Run("GET[success isAdmin=false]", func(t *testing.T) {
		uid := uint(1)
		streamId := uint(1234)
		courseId := uint(1111)
		chats := []model.Chat{
			{Message: "1", IsVisible: true},
			{Message: "2", IsVisible: true},
			{Message: "3", IsVisible: true},
		}

		w := httptest.NewRecorder()
		c, r := gin.CreateTestContext(w)

		r.Use(func(c *gin.Context) {
			c.Set("TUMLiveContext", tools.TUMLiveContext{User: &model.User{
				Model: gorm.Model{ID: uid},
				Name:  "Hansi",
				Role:  model.StudentType,
			}})
		})

		// chat mock
		chatMock := mock_dao.NewMockChatDao(gomock.NewController(t))
		chatMock.EXPECT().GetVisibleChats(uid, streamId).Return(chats, nil).AnyTimes()

		// streams mock
		streamsMock := mock_dao.NewMockStreamsDao(gomock.NewController(t))
		streamsMock.EXPECT().GetStreamByID(gomock.Any(), fmt.Sprintf("%d", streamId)).Return(model.Stream{
			Model:    gorm.Model{ID: streamId},
			CourseID: courseId,
		}, nil).AnyTimes()

		// course mock
		courseMock := mock_dao.NewMockCoursesDao(gomock.NewController(t))
		courseMock.EXPECT().GetCourseById(gomock.Any(), courseId).Return(model.Course{Visibility: "public"}, nil).AnyTimes()

		chat := r.Group("/api/chat")
		configGinChatRouter(chat, dao.DaoWrapper{ChatDao: chatMock, StreamsDao: streamsMock, CoursesDao: courseMock})

		c.Request, _ = http.NewRequest(http.MethodGet, fmt.Sprintf("/api/chat/%d/messages", streamId), nil)
		r.ServeHTTP(w, c.Request)

		j, _ := json.Marshal(chats)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, string(j), w.Body.String())
	})
}

func TestActivePoll(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Run("GET[no context]", func(t *testing.T) {
		streamId := "1234"

		w := httptest.NewRecorder()
		c, r := gin.CreateTestContext(w)

		chat := r.Group("/api/chat")
		configGinChatRouter(chat, dao.DaoWrapper{})

		c.Request, _ = http.NewRequest(http.MethodGet, fmt.Sprintf("/api/chat/%s/active-poll", streamId), nil)
		r.ServeHTTP(w, c.Request)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})

	t.Run("GET[success isAdmin=true]", func(t *testing.T) {
		gin.SetMode(gin.TestMode)

		uid := uint(1)
		streamId := uint(1234)
		courseId := uint(1111)
		pollId := uint(2)
		submitted := uint(1)
		poll := model.Poll{
			Model:    gorm.Model{ID: pollId},
			StreamID: streamId,
			Stream:   model.Stream{},
			Question: "1+1=?",
			Active:   true,
			PollOptions: []model.PollOption{
				{Model: gorm.Model{ID: 0}, Answer: "2", Votes: []model.User{}},
				{Model: gorm.Model{ID: 1}, Answer: "3", Votes: []model.User{}},
			},
		}

		w := httptest.NewRecorder()
		c, r := gin.CreateTestContext(w)

		r.Use(func(c *gin.Context) {
			c.Set("TUMLiveContext", tools.TUMLiveContext{User: &model.User{
				Model: gorm.Model{ID: uid},
				Name:  "Admin",
				Role:  model.AdminType,
			}})
		})

		// chat mock
		chatMock := mock_dao.NewMockChatDao(gomock.NewController(t))
		chatMock.EXPECT().GetActivePoll(streamId).Return(poll, nil).AnyTimes()
		chatMock.EXPECT().GetPollUserVote(pollId, uid).Return(submitted, nil).AnyTimes()
		chatMock.EXPECT().GetPollOptionVoteCount(gomock.Any()).Return(int64(1), nil).AnyTimes()

		// streams mock
		streamsMock := mock_dao.NewMockStreamsDao(gomock.NewController(t))
		streamsMock.EXPECT().GetStreamByID(gomock.Any(), fmt.Sprintf("%d", streamId)).Return(model.Stream{
			Model:    gorm.Model{ID: streamId},
			CourseID: courseId,
		}, nil).AnyTimes()

		// course mock
		courseMock := mock_dao.NewMockCoursesDao(gomock.NewController(t))
		courseMock.EXPECT().GetCourseById(gomock.Any(), courseId).Return(model.Course{Visibility: "public"}, nil).AnyTimes()

		chat := r.Group("/api/chat")
		configGinChatRouter(chat, dao.DaoWrapper{ChatDao: chatMock, StreamsDao: streamsMock, CoursesDao: courseMock})

		c.Request, _ = http.NewRequest(http.MethodGet, fmt.Sprintf("/api/chat/%d/active-poll", streamId), nil)
		r.ServeHTTP(w, c.Request)

		pollOptions := []gin.H{
			// Admin receives vote-count, Others don't
			{"ID": 0, "answer": "2", "votes": 1},
			{"ID": 1, "answer": "3", "votes": 1},
		}

		j, _ := json.Marshal(gin.H{
			"active":      true,
			"question":    poll.Question,
			"pollOptions": pollOptions,
			"submitted":   submitted,
		})

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, string(j), w.Body.String())
	})
}

func TestUsers(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Run("GET[no context]", func(t *testing.T) {
		streamId := "1234"

		w := httptest.NewRecorder()
		c, r := gin.CreateTestContext(w)

		chat := r.Group("/api/chat")
		configGinChatRouter(chat, dao.DaoWrapper{})

		c.Request, _ = http.NewRequest(http.MethodGet, fmt.Sprintf("/api/chat/%s/users", streamId), nil)
		r.ServeHTTP(w, c.Request)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})

	t.Run("GET[success]", func(t *testing.T) {
		uid := (uint)(1)
		streamId := (uint)(1234)
		courseId := (uint)(1111)
		users := []model.User{
			{Model: gorm.Model{ID: 1}, Name: "Wolfgang"},
			{Model: gorm.Model{ID: 1}, Name: "Omar"},
			{Model: gorm.Model{ID: 1}, Name: "Wilhelm"},
		}

		// this is copied from api/chat.go, we might consider moving this definition
		// out of the function for testing purposes.
		type chatUserSearchDto struct {
			ID   uint   `json:"id"`
			Name string `json:"name"`
		}
		usersResponse := []chatUserSearchDto{
			{ID: users[0].ID, Name: users[0].Name},
			{ID: users[1].ID, Name: users[1].Name},
			{ID: users[2].ID, Name: users[2].Name},
		}

		w := httptest.NewRecorder()
		c, r := gin.CreateTestContext(w)

		r.Use(func(c *gin.Context) {
			c.Set("TUMLiveContext", tools.TUMLiveContext{User: &model.User{
				Model: gorm.Model{ID: uid},
				Name:  "Admin",
				Role:  model.AdminType,
			}})
		})

		// chat mock
		chatMock := mock_dao.NewMockChatDao(gomock.NewController(t))
		chatMock.EXPECT().GetChatUsers(streamId).Return(users, nil).AnyTimes()

		// streams mock
		streamsMock := mock_dao.NewMockStreamsDao(gomock.NewController(t))
		streamsMock.EXPECT().GetStreamByID(gomock.Any(), fmt.Sprintf("%d", streamId)).Return(model.Stream{
			Model:    gorm.Model{ID: streamId},
			CourseID: courseId,
		}, nil).AnyTimes()

		// course mock
		courseMock := mock_dao.NewMockCoursesDao(gomock.NewController(t))
		courseMock.EXPECT().GetCourseById(gomock.Any(), courseId).Return(model.Course{}, nil).AnyTimes()

		chat := r.Group("/api/chat")
		configGinChatRouter(chat, dao.DaoWrapper{ChatDao: chatMock, StreamsDao: streamsMock, CoursesDao: courseMock})

		c.Request, _ = http.NewRequest(http.MethodGet, fmt.Sprintf("/api/chat/%d/users", streamId), nil)
		r.ServeHTTP(w, c.Request)

		j, _ := json.Marshal(usersResponse)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, string(j), w.Body.String())
	})
}
