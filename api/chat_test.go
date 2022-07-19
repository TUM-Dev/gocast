package api

import (
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/golang/mock/gomock"
	"github.com/joschahenningsen/TUM-Live/dao"
	"github.com/joschahenningsen/TUM-Live/mock_dao"
	"github.com/joschahenningsen/TUM-Live/model"
	"github.com/joschahenningsen/TUM-Live/tools/testutils"
	"gorm.io/gorm"
	"net/http"
	"testing"
)

func TestMessages(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Run("GET/api/chat/:streamID/messages", func(t *testing.T) {
		url := fmt.Sprintf("/api/chat/%d/messages", testutils.StreamFPVLive.ID)

		chats := []model.Chat{
			{Message: "1", IsVisible: true},
			{Message: "2", IsVisible: true},
			{Message: "3", IsVisible: true},
		}

		res, _ := json.Marshal(chats)

		testutils.TestCases{
			"no context": {
				Method:         http.MethodGet,
				Url:            url,
				TumLiveContext: nil,
				ExpectedCode:   http.StatusInternalServerError,
			},
			"success admin": {
				Method:         http.MethodGet,
				Url:            url,
				TumLiveContext: &testutils.TUMLiveContextAdmin,
				DaoWrapper: dao.DaoWrapper{
					ChatDao: func() dao.ChatDao {
						chatMock := mock_dao.NewMockChatDao(gomock.NewController(t))
						chatMock.
							EXPECT().
							GetAllChats(testutils.Admin.ID, testutils.StreamFPVLive.ID).
							Return(chats, nil).
							AnyTimes()
						return chatMock
					}(),
					StreamsDao: testutils.GetStreamMock(t),
					CoursesDao: testutils.GetCoursesMock(t),
				},
				ExpectedCode:     http.StatusOK,
				ExpectedResponse: res,
			},
			"success not admin": {
				Method:         http.MethodGet,
				Url:            url,
				TumLiveContext: &testutils.TUMLiveContextStudent,
				DaoWrapper: dao.DaoWrapper{
					ChatDao: func() dao.ChatDao {
						chatMock := mock_dao.NewMockChatDao(gomock.NewController(t))
						chatMock.
							EXPECT().
							GetVisibleChats(testutils.Student.ID, testutils.StreamFPVLive.ID).
							Return(chats, nil).
							AnyTimes()
						return chatMock
					}(),
					StreamsDao: testutils.GetStreamMock(t),
					CoursesDao: testutils.GetCoursesMock(t),
				},
				ExpectedCode:     http.StatusOK,
				ExpectedResponse: res,
			},
		}.Run(t, func(r *gin.Engine, wrapper dao.DaoWrapper) {
			configGinChatRouter(r.Group("/api/chat"), wrapper)
		})
	})
}

func TestActivePoll(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("GET/api/chat/:streamID/active-poll", func(t *testing.T) {
		url := fmt.Sprintf("/api/chat/%d/active-poll", testutils.StreamFPVLive.ID)

		submitted := uint(1)

		pollOptions := []gin.H{
			// Admin receives vote-count, Others don't
			{"ID": 0, "answer": "2", "votes": 1},
			{"ID": 1, "answer": "3", "votes": 1},
		}

		res, _ := json.Marshal(gin.H{
			"active":      true,
			"question":    testutils.PollStreamFPVLive.Question,
			"pollOptions": pollOptions,
			"submitted":   submitted,
		})

		testutils.TestCases{
			"no context": {
				Method:         http.MethodGet,
				Url:            url,
				TumLiveContext: nil,
				ExpectedCode:   http.StatusInternalServerError,
			},
			"success": {
				Method:         http.MethodGet,
				Url:            url,
				TumLiveContext: &testutils.TUMLiveContextAdmin,
				DaoWrapper: dao.DaoWrapper{
					ChatDao: func() dao.ChatDao {
						chatMock := mock_dao.NewMockChatDao(gomock.NewController(t))
						chatMock.
							EXPECT().
							GetActivePoll(testutils.StreamFPVLive.ID).
							Return(testutils.PollStreamFPVLive, nil).
							AnyTimes()
						chatMock.
							EXPECT().
							GetPollUserVote(testutils.PollStreamFPVLive.ID, testutils.Admin.ID).
							Return(submitted, nil).
							AnyTimes()
						chatMock.
							EXPECT().
							GetPollOptionVoteCount(gomock.Any()).
							Return(int64(1), nil).
							AnyTimes()
						return chatMock
					}(),
					StreamsDao: testutils.GetStreamMock(t),
					CoursesDao: testutils.GetCoursesMock(t),
				},
				ExpectedCode:     http.StatusOK,
				ExpectedResponse: res,
			},
		}.Run(t, func(r *gin.Engine, wrapper dao.DaoWrapper) {
			configGinChatRouter(r.Group("/api/chat"), wrapper)
		})
	})
}

func TestUsers(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("GET/api/chat/:streamID/users", func(t *testing.T) {
		url := fmt.Sprintf("/api/chat/%d/users", testutils.StreamFPVLive.ID)

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

		res, _ := json.Marshal(usersResponse)

		testutils.TestCases{
			"no context": {
				Method:         http.MethodGet,
				Url:            url,
				TumLiveContext: nil,
				ExpectedCode:   http.StatusInternalServerError,
			},
			"success": {
				Method:         http.MethodGet,
				Url:            url,
				TumLiveContext: &testutils.TUMLiveContextAdmin,
				DaoWrapper: dao.DaoWrapper{
					ChatDao: func() dao.ChatDao {
						chatMock := mock_dao.NewMockChatDao(gomock.NewController(t))
						chatMock.
							EXPECT().
							GetChatUsers(testutils.StreamFPVLive.ID).
							Return(users, nil).
							AnyTimes()
						return chatMock
					}(),
					StreamsDao: testutils.GetStreamMock(t),
					CoursesDao: testutils.GetCoursesMock(t),
				},
				ExpectedCode:     http.StatusOK,
				ExpectedResponse: res,
			},
		}.Run(t, func(r *gin.Engine, wrapper dao.DaoWrapper) {
			configGinChatRouter(r.Group("/api/chat"), wrapper)
		})
	})
}
