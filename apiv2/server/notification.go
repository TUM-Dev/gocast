package apiv2

import (
	"context"
	"net/http"

	e "github.com/TUM-Dev/gocast/apiv2/errors"
	h "github.com/TUM-Dev/gocast/apiv2/helpers"
	protobuf "github.com/TUM-Dev/gocast/apiv2/protobuf/server"
	"github.com/TUM-Dev/gocast/model"
	"google.golang.org/protobuf/types/known/emptypb"
)

// GetNotifications retrieves notifications for the current user.
func (a *API) GetNotifications(ctx context.Context, req *emptypb.Empty) (*protobuf.GetNotificationsResponse, error) {
	a.log.Info("GetNotifications")

	user, err := a.getCurrent(ctx)
	if err != nil {
		return nil, e.WithStatus(http.StatusUnauthorized, err)
	}

	targets := []model.NotificationTarget{model.TargetAll}
	if user != nil {
		targets = append(targets, model.TargetUser)
		switch user.Role {
		case model.AdminType:
			targets = append(targets, model.TargetAdmin)
		case model.LecturerType:
			targets = append(targets, model.TargetLecturer)
		case model.StudentType:
			targets = append(targets, model.TargetStudent)
		}
	}

	notifications, err := a.dao.NotificationsDao.GetNotifications(targets...)
	if err != nil {
		return nil, e.WithStatus(http.StatusBadRequest, err)
	}

	resp := &protobuf.GetNotificationsResponse{
		Notifications: make([]*protobuf.Notification, len(notifications)),
	}

	for i, notification := range notifications {
		resp.Notifications[i] = h.ParseNotificationToProto(notification)
	}

	return resp, nil
}

// GetServerNotifications retrieves current server notifications.
func (a *API) GetServerNotifications(ctx context.Context, req *emptypb.Empty) (*protobuf.GetServerNotificationsResponse, error) {
	a.log.Info("GetNotifications")

	notifications, err := a.dao.ServerNotificationDao.GetCurrentServerNotifications()
	if err != nil {
		return nil, e.WithStatus(http.StatusBadRequest, err)
	}

	resp := &protobuf.GetServerNotificationsResponse{
		ServerNotifications: make([]*protobuf.ServerNotification, len(notifications)),
	}

	for i, notification := range notifications {
		resp.ServerNotifications[i] = h.ParseServerNotificationToProto(notification)
	}

	return resp, nil
}
