package api

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/golang/mock/gomock"
	"github.com/joschahenningsen/TUM-Live/dao"
	"github.com/joschahenningsen/TUM-Live/mock_dao"
	"github.com/joschahenningsen/TUM-Live/model"
	"github.com/joschahenningsen/TUM-Live/tools"
	"github.com/joschahenningsen/TUM-Live/tools/testutils"
	"github.com/matthiasreumann/gomino"
	"gorm.io/gorm"
)

func ChatRouterWrapper(r *gin.Engine) {
	configGinChatRouter(r.Group("/api/chat"), dao.DaoWrapper{})
}

func TestMessages(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Run("GET/api/chat/:streamID/messages", func(t *testing.T) {
		url := fmt.Sprintf("/api/chat/%d/messages", testutils.StreamFPVLive.ID)

		chats := []model.Chat{
			{Message: "1", IsVisible: true},
			{Message: "2", IsVisible: true},
			{Message: "3", IsVisible: true},
		}

		gomino.TestCases{
			"no context": {
				Router:       ChatRouterWrapper,
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler),
				ExpectedCode: http.StatusInternalServerError,
			},
			"success admin": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
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
					}
					configGinChatRouter(r.Group("/api/chat"), wrapper)
				},
				Middlewares:      testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				ExpectedCode:     http.StatusOK,
				ExpectedResponse: chats,
			},
			"success not admin": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
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
					}
					configGinChatRouter(r.Group("/api/chat"), wrapper)
				},
				Middlewares:      testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextStudent)),
				ExpectedCode:     http.StatusOK,
				ExpectedResponse: chats,
			}}.
			Method(http.MethodGet).
			Url(url).
			Run(t, testutils.Equal)
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

		res := gin.H{
			"active":      true,
			"question":    testutils.PollStreamFPVLive.Question,
			"pollOptions": pollOptions,
			"submitted":   submitted,
		}

		gomino.TestCases{
			"no context": {
				Router:       ChatRouterWrapper,
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler),
				ExpectedCode: http.StatusInternalServerError,
			},
			"success": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
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
					}
					configGinChatRouter(r.Group("/api/chat"), wrapper)
				},
				Middlewares:      testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				ExpectedCode:     http.StatusOK,
				ExpectedResponse: res,
			}}.
			Method(http.MethodGet).
			Url(url).
			Run(t, testutils.Equal)
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

		gomino.TestCases{
			"no context": {
				Router:       ChatRouterWrapper,
				ExpectedCode: http.StatusInternalServerError,
			},
			"success": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
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
					}
					configGinChatRouter(r.Group("/api/chat"), wrapper)
				},
				Middlewares:      testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextStudent)),
				ExpectedCode:     http.StatusOK,
				ExpectedResponse: usersResponse,
			}}.
			Method(http.MethodGet).
			Url(url).
			Run(t, testutils.Equal)
	})
}
