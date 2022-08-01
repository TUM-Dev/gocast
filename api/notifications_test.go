package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/golang/mock/gomock"
	"github.com/joschahenningsen/TUM-Live/dao"
	"github.com/joschahenningsen/TUM-Live/mock_dao"
	"github.com/joschahenningsen/TUM-Live/model"
	"github.com/joschahenningsen/TUM-Live/tools"
	"github.com/joschahenningsen/TUM-Live/tools/testutils"
	"github.com/matthiasreumann/gomino"
	"net/http"
	"testing"
)

func NotificationsRouterWrapper(r *gin.Engine) {
	configNotificationsRouter(r, dao.DaoWrapper{})
}

func TestNotifications(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("GET/api/notifications", func(t *testing.T) {
		url := "/api/notifications/"

		notifications := []model.Notification{
			{SanitizedBody: "Brand new features!"},
			{SanitizedBody: "Brand new features!"},
		}

		notificationDao := func(targets []model.NotificationTarget) dao.NotificationsDao {
			notificationsMock := mock_dao.NewMockNotificationsDao(gomock.NewController(t))
			notificationsMock.
				EXPECT().
				GetNotifications(targets).
				Return(notifications, nil).
				AnyTimes()
			return notificationsMock
		}

		gomino.TestCases{
			"can not get notifications": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						NotificationsDao: func() dao.NotificationsDao {
							notificationsMock := mock_dao.NewMockNotificationsDao(gomock.NewController(t))
							notificationsMock.
								EXPECT().
								GetNotifications([]model.NotificationTarget{model.TargetAll}).
								Return([]model.Notification{}, errors.New("")).
								AnyTimes()
							return notificationsMock
						}(),
					}
					configNotificationsRouter(r, wrapper)
				},
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextEmpty)),
				ExpectedCode: http.StatusNotFound,
			},
			"success not logged in": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						NotificationsDao: notificationDao([]model.NotificationTarget{model.TargetAll}),
					}
					configNotificationsRouter(r, wrapper)
				},
				Middlewares:      testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextEmpty)),
				ExpectedCode:     http.StatusOK,
				ExpectedResponse: notifications,
			},
			"success student": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						NotificationsDao: notificationDao([]model.NotificationTarget{model.TargetAll, model.TargetUser, model.TargetStudent}),
					}
					configNotificationsRouter(r, wrapper)
				},
				Middlewares:      testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextStudent)),
				ExpectedCode:     http.StatusOK,
				ExpectedResponse: notifications,
			},
			"success lecturer": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						NotificationsDao: notificationDao([]model.NotificationTarget{model.TargetAll, model.TargetUser, model.TargetLecturer}),
					}
					configNotificationsRouter(r, wrapper)
				},
				Middlewares:      testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextLecturer)),
				ExpectedCode:     http.StatusOK,
				ExpectedResponse: notifications,
			},
			"success admin": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						NotificationsDao: notificationDao([]model.NotificationTarget{model.TargetAll, model.TargetUser, model.TargetAdmin}),
					}
					configNotificationsRouter(r, wrapper)
				},
				Middlewares:      testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				ExpectedCode:     http.StatusOK,
				ExpectedResponse: notifications,
			}}.
			Method(http.MethodGet).
			Url(url).
			Run(t, testutils.Equal)
	})

	t.Run("POST/api/notifications/", func(t *testing.T) {
		url := "/api/notifications/"

		title := "Now!"
		notification := model.Notification{
			Title:         &title,
			SanitizedBody: "Brand new Features!",
		}

		gomino.TestCases{
			"invalid body": {
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				ExpectedCode: http.StatusBadRequest,
			},
			"can not add notification": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						NotificationsDao: func() dao.NotificationsDao {
							mock := mock_dao.NewMockNotificationsDao(gomock.NewController(t))
							notification.Body = notification.SanitizedBody // reverse json binding here too
							mock.
								EXPECT().
								AddNotification(gomock.Any(), &notification).
								Return(errors.New("")).
								AnyTimes()
							return mock
						}(),
					}
					configNotificationsRouter(r, wrapper)
				},
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				Body:         notification,
				ExpectedCode: http.StatusInternalServerError,
			},
			"success": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						NotificationsDao: func() dao.NotificationsDao {
							mock := mock_dao.NewMockNotificationsDao(gomock.NewController(t))
							notification.Body = notification.SanitizedBody // reverse json binding here too
							mock.
								EXPECT().
								AddNotification(gomock.Any(), &notification).
								Return(nil).
								AnyTimes()
							return mock
						}(),
					}
					configNotificationsRouter(r, wrapper)
				},
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				Body:         notification,
				ExpectedCode: http.StatusOK,
			}}.
			Router(NotificationsRouterWrapper).
			Method(http.MethodPost).
			Url(url).
			Run(t, testutils.Equal)
	})

	t.Run("DELETE/api/notifications/:id", func(t *testing.T) {
		id := uint(1)
		url := fmt.Sprintf("/api/notifications/%d", id)

		res, _ := json.Marshal(gin.H{"success": true})

		gomino.TestCases{
			"invalid id": {
				Url:          "/api/notifications/abc",
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				ExpectedCode: http.StatusBadRequest,
			},
			"can not delete notification": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						NotificationsDao: func() dao.NotificationsDao {
							mock := mock_dao.NewMockNotificationsDao(gomock.NewController(t))
							mock.
								EXPECT().
								DeleteNotification(gomock.Any(), id).
								Return(errors.New("")).
								AnyTimes()
							return mock
						}(),
					}
					configNotificationsRouter(r, wrapper)
				},
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				ExpectedCode: http.StatusInternalServerError,
			},
			"success": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						NotificationsDao: func() dao.NotificationsDao {
							mock := mock_dao.NewMockNotificationsDao(gomock.NewController(t))
							mock.
								EXPECT().
								DeleteNotification(gomock.Any(), id).
								Return(nil).
								AnyTimes()
							return mock
						}(),
					}
					configNotificationsRouter(r, wrapper)
				},
				Middlewares:      testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				ExpectedCode:     http.StatusOK,
				ExpectedResponse: res,
			}}.
			Router(NotificationsRouterWrapper).
			Method(http.MethodDelete).
			Url(url).
			Run(t, testutils.Equal)
	})
}
