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
	"github.com/joschahenningsen/TUM-Live/tools/testutils"
	"net/http"
	"testing"
)

func TestNotifications(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("GET/api/notifications", func(t *testing.T) {
		url := "/api/notifications/"

		notifications := []model.Notification{
			{SanitizedBody: "Brand new features!"},
			{SanitizedBody: "Brand new features!"},
		}

		res, _ := json.Marshal(notifications)

		notificationDao := func(targets []model.NotificationTarget) dao.NotificationsDao {
			notificationsMock := mock_dao.NewMockNotificationsDao(gomock.NewController(t))
			notificationsMock.
				EXPECT().
				GetNotifications(targets).
				Return(notifications, nil).
				AnyTimes()
			return notificationsMock
		}

		testutils.TestCases{
			"can not get notifications": {
				Method: http.MethodGet,
				Url:    url,
				DaoWrapper: dao.DaoWrapper{
					NotificationsDao: func() dao.NotificationsDao {
						notificationsMock := mock_dao.NewMockNotificationsDao(gomock.NewController(t))
						notificationsMock.
							EXPECT().
							GetNotifications([]model.NotificationTarget{model.TargetAll}).
							Return([]model.Notification{}, errors.New("")).
							AnyTimes()
						return notificationsMock
					}(),
				},
				TumLiveContext: &testutils.TUMLiveContextEmpty,
				ExpectedCode:   http.StatusNotFound,
			},
			"success not logged in": {
				Method: http.MethodGet,
				Url:    url,
				DaoWrapper: dao.DaoWrapper{
					NotificationsDao: notificationDao([]model.NotificationTarget{model.TargetAll}),
				},
				TumLiveContext:   &testutils.TUMLiveContextEmpty,
				ExpectedCode:     http.StatusOK,
				ExpectedResponse: res,
			},
			"success admin": {
				Method: http.MethodGet,
				Url:    url,
				DaoWrapper: dao.DaoWrapper{
					NotificationsDao: notificationDao([]model.NotificationTarget{model.TargetAll, model.TargetUser, model.TargetAdmin}),
				},
				TumLiveContext:   &testutils.TUMLiveContextAdmin,
				ExpectedCode:     http.StatusOK,
				ExpectedResponse: res,
			},
			"success lecturer": {
				Method: http.MethodGet,
				Url:    url,
				DaoWrapper: dao.DaoWrapper{
					NotificationsDao: notificationDao([]model.NotificationTarget{model.TargetAll, model.TargetUser, model.TargetLecturer}),
				},
				TumLiveContext:   &testutils.TUMLiveContextAdmin,
				ExpectedCode:     http.StatusOK,
				ExpectedResponse: res,
			},
			"success student": {
				Method: http.MethodGet,
				Url:    url,
				DaoWrapper: dao.DaoWrapper{
					NotificationsDao: notificationDao([]model.NotificationTarget{model.TargetAll, model.TargetUser, model.TargetStudent}),
				},
				TumLiveContext:   &testutils.TUMLiveContextAdmin,
				ExpectedCode:     http.StatusOK,
				ExpectedResponse: res,
			},
		}.Run(t, configNotificationsRouter)
	})

	t.Run("POST/api/notifications/", func(t *testing.T) {
		url := "/api/notifications/"

		title := "Now!"
		notification := model.Notification{
			Title:         &title,
			SanitizedBody: "Brand new Features!",
		}

		testutils.TestCases{
			"invalid body": {
				Method:       http.MethodPost,
				Url:          url,
				DaoWrapper:   dao.DaoWrapper{},
				Body:         nil,
				ExpectedCode: http.StatusBadRequest,
			},
			"can not add notification": {
				Method: http.MethodPost,
				Url:    url,
				DaoWrapper: dao.DaoWrapper{
					NotificationsDao: func() dao.NotificationsDao {
						mock := mock_dao.NewMockNotificationsDao(gomock.NewController(t))
						notification.Body = notification.SanitizedBody // reverse json binding here too
						mock.
							EXPECT().
							AddNotification(&notification).
							Return(errors.New("")).
							AnyTimes()
						return mock
					}(),
				},
				Body:         notification,
				ExpectedCode: http.StatusInternalServerError,
			},
			"success": {
				Method: http.MethodPost,
				Url:    url,
				DaoWrapper: dao.DaoWrapper{
					NotificationsDao: func() dao.NotificationsDao {
						mock := mock_dao.NewMockNotificationsDao(gomock.NewController(t))
						notification.Body = notification.SanitizedBody // reverse json binding here too
						mock.
							EXPECT().
							AddNotification(&notification).
							Return(nil).
							AnyTimes()
						return mock
					}(),
				},
				Body:         notification,
				ExpectedCode: http.StatusOK,
			},
		}.Run(t, configNotificationsRouter)
	})

	t.Run("DELETE/api/notifications/:id", func(t *testing.T) {
		id := uint(1)
		url := fmt.Sprintf("/api/notifications/%d", id)

		res, _ := json.Marshal(gin.H{"success": true})

		testutils.TestCases{
			"invalid id": {
				Method:       http.MethodDelete,
				Url:          "/api/notifications/abc",
				ExpectedCode: http.StatusBadRequest,
			},
			"can not delete notification": {
				Method: http.MethodDelete,
				Url:    url,
				DaoWrapper: dao.DaoWrapper{
					NotificationsDao: func() dao.NotificationsDao {
						mock := mock_dao.NewMockNotificationsDao(gomock.NewController(t))
						mock.
							EXPECT().
							DeleteNotification(id).
							Return(errors.New("")).
							AnyTimes()
						return mock
					}(),
				},
				ExpectedCode: http.StatusInternalServerError,
			},
			"success": {
				Method: http.MethodDelete,
				Url:    url,
				DaoWrapper: dao.DaoWrapper{
					NotificationsDao: func() dao.NotificationsDao {
						mock := mock_dao.NewMockNotificationsDao(gomock.NewController(t))
						mock.
							EXPECT().
							DeleteNotification(id).
							Return(nil).
							AnyTimes()
						return mock
					}(),
				},
				ExpectedCode:     http.StatusOK,
				ExpectedResponse: res,
			},
		}.Run(t, configNotificationsRouter)
	})
}
