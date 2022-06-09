package api

import (
	"bytes"
	"encoding/json"
	"errors"
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

func TestNotifications(t *testing.T) {
	t.Run("GET[GetNotifications returns error]", func(t *testing.T) {
		notificationsMock := mock_dao.NewMockNotificationsDao(gomock.NewController(t))

		w := httptest.NewRecorder()
		c, r := gin.CreateTestContext(w)
		r.Use(func(c *gin.Context) {
			c.Set("TUMLiveContext", tools.TUMLiveContext{})
		})
		configNotificationsRouter(r, dao.DaoWrapper{NotificationsDao: notificationsMock})

		targets := []model.NotificationTarget{model.TargetAll}

		notificationsMock.
			EXPECT().
			GetNotifications(targets).
			Return([]model.Notification{}, errors.New("")).
			AnyTimes()

		c.Request, _ = http.NewRequest(http.MethodGet, "/api/notifications/", nil)
		r.ServeHTTP(w, c.Request)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("GET[success not logged in]", func(t *testing.T) {
		notificationsMock := mock_dao.NewMockNotificationsDao(gomock.NewController(t))

		w := httptest.NewRecorder()
		c, r := gin.CreateTestContext(w)
		r.Use(func(c *gin.Context) {
			c.Set("TUMLiveContext", tools.TUMLiveContext{User: nil})
		})
		configNotificationsRouter(r, dao.DaoWrapper{NotificationsDao: notificationsMock})

		notifications := []model.Notification{
			{SanitizedBody: "Brand new features!"},
			{SanitizedBody: "Brand new features!"},
		}

		targets := []model.NotificationTarget{model.TargetAll}

		notificationsMock.
			EXPECT().
			GetNotifications(targets).
			Return(notifications, nil).
			AnyTimes()

		c.Request, _ = http.NewRequest(http.MethodGet, "/api/notifications/", nil)
		r.ServeHTTP(w, c.Request)

		j, _ := json.Marshal(notifications)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, string(j), w.Body.String())
	})

	t.Run("GET[success logged in]", func(t *testing.T) {
		notificationsMock := mock_dao.NewMockNotificationsDao(gomock.NewController(t))

		w := httptest.NewRecorder()
		c, r := gin.CreateTestContext(w)
		r.Use(func(c *gin.Context) {
			c.Set("TUMLiveContext", tools.TUMLiveContext{User: &model.User{
				Role: model.AdminType,
			}})
		})
		configNotificationsRouter(r, dao.DaoWrapper{NotificationsDao: notificationsMock})

		notifications := []model.Notification{
			{SanitizedBody: "Brand new features!"},
			{SanitizedBody: "Brand new features!"},
		}

		targets := []model.NotificationTarget{model.TargetAll, model.TargetUser, model.TargetAdmin}

		notificationsMock.
			EXPECT().
			GetNotifications(targets).
			Return(notifications, nil).
			AnyTimes()

		c.Request, _ = http.NewRequest(http.MethodGet, "/api/notifications/", nil)
		r.ServeHTTP(w, c.Request)

		j, _ := json.Marshal(notifications)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, string(j), w.Body.String())
	})

	t.Run("POST[Invalid Body]", func(t *testing.T) {
		notificationsMock := mock_dao.NewMockNotificationsDao(gomock.NewController(t))

		w := httptest.NewRecorder()
		c, r := gin.CreateTestContext(w)
		configNotificationsRouter(r, dao.DaoWrapper{NotificationsDao: notificationsMock})

		invalidJson := bytes.NewBufferString("{")

		c.Request, _ = http.NewRequest(http.MethodPost, "/api/notifications/", invalidJson)
		r.ServeHTTP(w, c.Request)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("POST[AddNotification returns error]", func(t *testing.T) {
		notificationsMock := mock_dao.NewMockNotificationsDao(gomock.NewController(t))

		w := httptest.NewRecorder()
		c, r := gin.CreateTestContext(w)
		configNotificationsRouter(r, dao.DaoWrapper{NotificationsDao: notificationsMock})

		title := "Now!"
		notification := model.Notification{
			Title:         &title,
			SanitizedBody: "Brand new Features!",
		}
		j, _ := json.Marshal(notification)
		body := bytes.NewBuffer(j)

		notification.Body = notification.SanitizedBody // reverse json binding here too
		notificationsMock.
			EXPECT().
			AddNotification(&notification).Return(errors.New("")).AnyTimes()

		c.Request, _ = http.NewRequest(http.MethodPost, "/api/notifications/", body)
		r.ServeHTTP(w, c.Request)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})

	t.Run("POST[success]", func(t *testing.T) {
		notificationsMock := mock_dao.NewMockNotificationsDao(gomock.NewController(t))

		w := httptest.NewRecorder()
		c, r := gin.CreateTestContext(w)
		configNotificationsRouter(r, dao.DaoWrapper{NotificationsDao: notificationsMock})

		title := "Now!"
		notification := model.Notification{
			Title:         &title,
			SanitizedBody: "Brand new Features!",
		}
		req, _ := json.Marshal(notification)

		notification.Body = notification.SanitizedBody // reverse json binding here too
		notificationsMock.
			EXPECT().
			AddNotification(&notification).Return(nil).AnyTimes()

		c.Request, _ = http.NewRequest(http.MethodPost, "/api/notifications/", bytes.NewBuffer(req))
		r.ServeHTTP(w, c.Request)

		jResponse, _ := json.Marshal(notification)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, string(jResponse), w.Body.String())
	})

	t.Run("DELETE[invalid parameter 'id']", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, r := gin.CreateTestContext(w)
		configNotificationsRouter(r, dao.DaoWrapper{})

		c.Request, _ = http.NewRequest(http.MethodDelete, "/api/notifications/abc", nil)
		r.ServeHTTP(w, c.Request)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("DELETE[DeleteNotification returns error]", func(t *testing.T) {
		notificationsMock := mock_dao.NewMockNotificationsDao(gomock.NewController(t))

		id := uint(1)

		w := httptest.NewRecorder()
		c, r := gin.CreateTestContext(w)
		configNotificationsRouter(r, dao.DaoWrapper{NotificationsDao: notificationsMock})

		notificationsMock.
			EXPECT().
			DeleteNotification(id).
			Return(errors.New("")).
			AnyTimes()

		c.Request, _ = http.NewRequest(http.MethodDelete,
			fmt.Sprintf("/api/notifications/%d", id), nil)
		r.ServeHTTP(w, c.Request)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})

	t.Run("DELETE[success]", func(t *testing.T) {
		notificationsMock := mock_dao.NewMockNotificationsDao(gomock.NewController(t))

		id := uint(1)

		w := httptest.NewRecorder()
		c, r := gin.CreateTestContext(w)
		configNotificationsRouter(r, dao.DaoWrapper{NotificationsDao: notificationsMock})

		notificationsMock.
			EXPECT().
			DeleteNotification(id).
			Return(nil).
			AnyTimes()

		c.Request, _ = http.NewRequest(http.MethodDelete,
			fmt.Sprintf("/api/notifications/%d", id), nil)
		r.ServeHTTP(w, c.Request)

		j, _ := json.Marshal(gin.H{"success": true})

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, string(j), w.Body.String())
	})
}
