package api

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/golang/mock/gomock"
	"github.com/joschahenningsen/TUM-Live/dao"
	"github.com/joschahenningsen/TUM-Live/mock_dao"
	"github.com/joschahenningsen/TUM-Live/model"
	"github.com/joschahenningsen/TUM-Live/tools"
	"github.com/stretchr/testify/assert"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetMessages_noContext(t *testing.T) {
	gin.SetMode(gin.TestMode)

	streamId := (uint)(1234)

	w := httptest.NewRecorder()
	c, r := gin.CreateTestContext(w)

	chat := r.Group("/api/chat")
	configGinChatRouter(chat, dao.DaoWrapper{})
	c.Request, _ = http.NewRequest(http.MethodGet, fmt.Sprintf("/api/chat/%d/messages", streamId), nil)
	r.ServeHTTP(w, c.Request)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestGetMessages_isAdmin(t *testing.T) {
	gin.SetMode(gin.TestMode)

	uid := (uint)(1)
	streamId := (uint)(1234)
	courseId := (uint)(1111)
	allMessages := []model.Chat{
		{Message: "1", IsVisible: false},
		{Message: "2", IsVisible: true},
		{Message: "3", IsVisible: true},
	}

	w := httptest.NewRecorder()
	c, r := gin.CreateTestContext(w)

	r.Use(func(c *gin.Context) {
		c.Set("TUMLiveContext", tools.TUMLiveContext{User: &model.User{
			Name: "Admin",
			Role: model.AdminType,
		}})
	})

	// chat mock
	chatMock := mock_dao.NewMockChatDao(gomock.NewController(t))
	chatMock.EXPECT().GetAllChats(uid, streamId).Return(allMessages, nil).AnyTimes()

	// streams mock
	streamsMock := mock_dao.NewMockStreamsDao(gomock.NewController(t))
	streamsMock.EXPECT().GetStreamByID(gomock.Any(), fmt.Sprintf("%d", streamId)).Return(model.Stream{
		CourseID: courseId,
	}, nil).AnyTimes()

	// course mock
	courseMock := mock_dao.NewMockCoursesDao(gomock.NewController(t))
	courseMock.EXPECT().GetCourseById(gomock.Any(), courseId).Return(model.Course{
		UserID:           1,
		DownloadsEnabled: false,
	}, nil).AnyTimes()

	chat := r.Group("/api/chat")
	configGinChatRouter(chat, dao.DaoWrapper{ChatDao: chatMock})

	c.Request, _ = http.NewRequest(http.MethodGet, fmt.Sprintf("/api/chat/%d/messages", streamId), nil)
	r.ServeHTTP(w, c.Request)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestGetActivePoll_noContext(t *testing.T) {
	gin.SetMode(gin.TestMode)

	streamId := "1234"

	w := httptest.NewRecorder()
	c, r := gin.CreateTestContext(w)

	chat := r.Group("/api/chat")
	configGinChatRouter(chat, dao.DaoWrapper{})

	c.Request, _ = http.NewRequest(http.MethodGet, fmt.Sprintf("/api/chat/%s/active-poll", streamId), nil)
	r.ServeHTTP(w, c.Request)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestGetUsers_noContext(t *testing.T) {
	gin.SetMode(gin.TestMode)

	streamId := "1234"

	w := httptest.NewRecorder()
	c, r := gin.CreateTestContext(w)

	chat := r.Group("/api/chat")
	configGinChatRouter(chat, dao.DaoWrapper{})

	c.Request, _ = http.NewRequest(http.MethodGet, fmt.Sprintf("/api/chat/%s/users", streamId), nil)
	r.ServeHTTP(w, c.Request)

	assert.Equal(t, http.StatusNotFound, w.Code)
}
