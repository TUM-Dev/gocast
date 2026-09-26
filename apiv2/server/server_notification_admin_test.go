package apiv2

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"go.uber.org/mock/gomock"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
	"gorm.io/gorm"

	protobuf "github.com/TUM-Dev/gocast/apiv2/protobuf/server"
	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/mock_dao"
	"github.com/TUM-Dev/gocast/model"
)

func TestListServerNotificationsAdmin(t *testing.T) {
	ctrl := gomock.NewController(t)
	notificationMock := mock_dao.NewMockServerNotificationDao(ctrl)
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	expires := time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)
	notificationMock.EXPECT().GetAllServerNotifications().Return([]model.ServerNotification{
		{Model: gorm.Model{ID: 1}, Text: "Maintenance", Warn: false, Start: start, Expires: expires},
	}, nil).Times(1)

	api := &API{dao: dao.DaoWrapper{ServerNotificationDao: notificationMock}, log: slog.Default()}

	resp, err := api.ListServerNotificationsAdmin(context.Background(), &emptypb.Empty{})
	if err != nil {
		t.Fatalf("ListServerNotificationsAdmin: %v", err)
	}
	if len(resp.Notifications) != 1 || resp.Notifications[0].Id != 1 {
		t.Fatalf("got %+v, want one notification with id 1", resp.Notifications)
	}
	// GetAllServerNotifications, not GetCurrentServerNotifications: the admin list
	// must show past and future notifications too, for editing.
	if resp.Notifications[0].Text != "Maintenance" {
		t.Errorf("text = %q, want %q", resp.Notifications[0].Text, "Maintenance")
	}
}

func TestCreateServerNotification(t *testing.T) {
	t.Run("creates a notification and returns it with its assigned id", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		notificationMock := mock_dao.NewMockServerNotificationDao(ctrl)
		notificationMock.EXPECT().CreateServerNotification(gomock.Any()).DoAndReturn(
			func(notification *model.ServerNotification) error {
				notification.ID = 7
				return nil
			}).Times(1)

		api := &API{dao: dao.DaoWrapper{ServerNotificationDao: notificationMock}, log: slog.Default()}

		start := timestamppb.New(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
		expires := timestamppb.New(time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC))
		resp, err := api.CreateServerNotification(context.Background(), &protobuf.CreateServerNotificationRequest{
			Text: "Maintenance", Warn: true, Start: start, Expires: expires,
		})
		if err != nil {
			t.Fatalf("CreateServerNotification: %v", err)
		}
		if resp.Id != 7 || resp.Text != "Maintenance" || !resp.Warn {
			t.Errorf("got %+v", resp)
		}
	})

	t.Run("rejects a blank message", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		notificationMock := mock_dao.NewMockServerNotificationDao(ctrl)
		notificationMock.EXPECT().CreateServerNotification(gomock.Any()).Times(0)

		api := &API{dao: dao.DaoWrapper{ServerNotificationDao: notificationMock}, log: slog.Default()}

		_, err := api.CreateServerNotification(context.Background(), &protobuf.CreateServerNotificationRequest{
			Text: "",
		})
		if err == nil {
			t.Fatal("a blank message was accepted")
		}
	})

	t.Run("rejects expires before start", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		notificationMock := mock_dao.NewMockServerNotificationDao(ctrl)
		notificationMock.EXPECT().CreateServerNotification(gomock.Any()).Times(0)

		api := &API{dao: dao.DaoWrapper{ServerNotificationDao: notificationMock}, log: slog.Default()}

		start := timestamppb.New(time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC))
		expires := timestamppb.New(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
		_, err := api.CreateServerNotification(context.Background(), &protobuf.CreateServerNotificationRequest{
			Text: "Maintenance", Start: start, Expires: expires,
		})
		if err == nil {
			t.Fatal("expires before start was accepted")
		}
	})
}

func TestUpdateServerNotification(t *testing.T) {
	t.Run("updates an existing notification", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		notificationMock := mock_dao.NewMockServerNotificationDao(ctrl)
		notificationMock.EXPECT().UpdateServerNotification(gomock.Any(), "3").Return(nil).Times(1)

		api := &API{dao: dao.DaoWrapper{ServerNotificationDao: notificationMock}, log: slog.Default()}

		start := timestamppb.New(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
		expires := timestamppb.New(time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC))
		resp, err := api.UpdateServerNotification(context.Background(), &protobuf.UpdateServerNotificationRequest{
			Id: 3, Text: "Updated", Warn: false, Start: start, Expires: expires,
		})
		if err != nil {
			t.Fatalf("UpdateServerNotification: %v", err)
		}
		if resp.Id != 3 || resp.Text != "Updated" {
			t.Errorf("got %+v", resp)
		}
	})

	t.Run("rejects expires before start", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		notificationMock := mock_dao.NewMockServerNotificationDao(ctrl)
		notificationMock.EXPECT().UpdateServerNotification(gomock.Any(), gomock.Any()).Times(0)

		api := &API{dao: dao.DaoWrapper{ServerNotificationDao: notificationMock}, log: slog.Default()}

		start := timestamppb.New(time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC))
		expires := timestamppb.New(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
		_, err := api.UpdateServerNotification(context.Background(), &protobuf.UpdateServerNotificationRequest{
			Id: 3, Text: "Updated", Start: start, Expires: expires,
		})
		if err == nil {
			t.Fatal("expires before start was accepted")
		}
	})

	t.Run("surfaces a dao failure", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		notificationMock := mock_dao.NewMockServerNotificationDao(ctrl)
		notificationMock.EXPECT().UpdateServerNotification(gomock.Any(), "3").Return(errors.New("boom")).Times(1)

		api := &API{dao: dao.DaoWrapper{ServerNotificationDao: notificationMock}, log: slog.Default()}

		start := timestamppb.New(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
		expires := timestamppb.New(time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC))
		_, err := api.UpdateServerNotification(context.Background(), &protobuf.UpdateServerNotificationRequest{
			Id: 3, Text: "Updated", Start: start, Expires: expires,
		})
		if err == nil {
			t.Fatal("a dao error was swallowed")
		}
	})
}

func TestDeleteServerNotification(t *testing.T) {
	ctrl := gomock.NewController(t)
	notificationMock := mock_dao.NewMockServerNotificationDao(ctrl)
	notificationMock.EXPECT().DeleteServerNotification("3").Return(nil).Times(1)

	api := &API{dao: dao.DaoWrapper{ServerNotificationDao: notificationMock}, log: slog.Default()}

	if _, err := api.DeleteServerNotification(context.Background(), &protobuf.DeleteServerNotificationRequest{Id: 3}); err != nil {
		t.Fatalf("DeleteServerNotification: %v", err)
	}
}
