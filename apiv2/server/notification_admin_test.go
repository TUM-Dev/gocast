package apiv2

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"go.uber.org/mock/gomock"
	"google.golang.org/protobuf/types/known/emptypb"

	protobuf "github.com/TUM-Dev/gocast/apiv2/protobuf/server"
	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/mock_dao"
	"github.com/TUM-Dev/gocast/model"
)

func title(s string) *string { return &s }

func TestListNotificationsAdmin(t *testing.T) {
	ctrl := gomock.NewController(t)
	notificationsMock := mock_dao.NewMockNotificationsDao(ctrl)
	notificationsMock.EXPECT().GetAllNotifications().Return([]model.Notification{
		{Model: model.Model{ID: 1}, Title: title("Maintenance"), Body: "planned downtime", Target: model.TargetAll},
	}, nil).Times(1)

	api := &API{dao: dao.DaoWrapper{NotificationsDao: notificationsMock}, log: slog.Default()}

	resp, err := api.ListNotificationsAdmin(context.Background(), &emptypb.Empty{})
	if err != nil {
		t.Fatalf("ListNotificationsAdmin: %v", err)
	}
	if len(resp.Notifications) != 1 {
		t.Fatalf("got %d notifications, want 1", len(resp.Notifications))
	}
	if resp.Notifications[0].Id != 1 || resp.Notifications[0].Title != "Maintenance" {
		t.Errorf("got %+v", resp.Notifications[0])
	}
	// Unfiltered: an administrator manages the whole history, not just what would be
	// shown to them personally.
	if resp.Notifications[0].Target != uint32(model.TargetAll) {
		t.Errorf("target = %d, want %d", resp.Notifications[0].Target, model.TargetAll)
	}
}

func TestCreateNotification(t *testing.T) {
	t.Run("broadcasts a notification with a title", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		notificationsMock := mock_dao.NewMockNotificationsDao(ctrl)
		notificationsMock.EXPECT().AddNotification(gomock.Any()).DoAndReturn(func(n *model.Notification) error {
			n.ID = 5
			return nil
		}).Times(1)

		api := &API{dao: dao.DaoWrapper{NotificationsDao: notificationsMock}, log: slog.Default()}

		resp, err := api.CreateNotification(context.Background(), &protobuf.CreateNotificationRequest{
			Title: "Heads up", Body: "**bold** text", Target: uint32(model.TargetAdmin),
		})
		if err != nil {
			t.Fatalf("CreateNotification: %v", err)
		}
		if resp.Id != 5 || resp.Title != "Heads up" {
			t.Errorf("got %+v", resp)
		}
		if resp.Target != uint32(model.TargetAdmin) {
			t.Errorf("target = %d, want %d", resp.Target, model.TargetAdmin)
		}
		// The body comes back sanitized HTML, the same shape a later list call
		// returns, not the raw Markdown that was sent.
		if resp.Body == "**bold** text" {
			t.Errorf("body was not rendered from Markdown: %q", resp.Body)
		}
	})

	t.Run("treats an empty title as no title", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		notificationsMock := mock_dao.NewMockNotificationsDao(ctrl)
		notificationsMock.EXPECT().AddNotification(gomock.Any()).DoAndReturn(func(n *model.Notification) error {
			if n.Title != nil {
				t.Errorf("title = %v, want nil", *n.Title)
			}
			return nil
		}).Times(1)

		api := &API{dao: dao.DaoWrapper{NotificationsDao: notificationsMock}, log: slog.Default()}

		if _, err := api.CreateNotification(context.Background(), &protobuf.CreateNotificationRequest{
			Body: "no title here", Target: uint32(model.TargetAll),
		}); err != nil {
			t.Fatalf("CreateNotification: %v", err)
		}
	})

	t.Run("rejects a target outside the known range", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		notificationsMock := mock_dao.NewMockNotificationsDao(ctrl)
		notificationsMock.EXPECT().AddNotification(gomock.Any()).Times(0)

		api := &API{dao: dao.DaoWrapper{NotificationsDao: notificationsMock}, log: slog.Default()}

		_, err := api.CreateNotification(context.Background(), &protobuf.CreateNotificationRequest{
			Body: "body", Target: 99,
		})
		if err == nil {
			t.Fatal("an out-of-range target was accepted")
		}
	})

	t.Run("reports a failed create", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		notificationsMock := mock_dao.NewMockNotificationsDao(ctrl)
		notificationsMock.EXPECT().AddNotification(gomock.Any()).Return(errors.New("database is on fire")).Times(1)

		api := &API{dao: dao.DaoWrapper{NotificationsDao: notificationsMock}, log: slog.Default()}

		_, err := api.CreateNotification(context.Background(), &protobuf.CreateNotificationRequest{
			Body: "body", Target: uint32(model.TargetAll),
		})
		if err == nil {
			t.Fatal("a failed create was reported as success")
		}
	})
}

func TestDeleteNotification(t *testing.T) {
	t.Run("deletes the notification it was given", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		notificationsMock := mock_dao.NewMockNotificationsDao(ctrl)
		notificationsMock.EXPECT().DeleteNotification(uint(3)).Return(nil).Times(1)

		api := &API{dao: dao.DaoWrapper{NotificationsDao: notificationsMock}, log: slog.Default()}

		if _, err := api.DeleteNotification(context.Background(), &protobuf.DeleteNotificationRequest{Id: 3}); err != nil {
			t.Fatalf("DeleteNotification: %v", err)
		}
	})

	t.Run("reports a failed delete", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		notificationsMock := mock_dao.NewMockNotificationsDao(ctrl)
		notificationsMock.EXPECT().DeleteNotification(gomock.Any()).Return(errors.New("database is on fire")).Times(1)

		api := &API{dao: dao.DaoWrapper{NotificationsDao: notificationsMock}, log: slog.Default()}

		_, err := api.DeleteNotification(context.Background(), &protobuf.DeleteNotificationRequest{Id: 3})
		if err == nil {
			t.Fatal("a failed delete was reported as success")
		}
	})
}
