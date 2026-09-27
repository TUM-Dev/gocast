package apiv2

import (
	"context"
	"errors"
	"net/http"

	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	e "github.com/TUM-Dev/gocast/apiv2/errors"
	protobuf "github.com/TUM-Dev/gocast/apiv2/protobuf/server"
	"github.com/TUM-Dev/gocast/model"
)

// Every RPC here is gated on PermAdministerServer by its policy in services.go.

// ListNotificationsAdmin lists every notification ever broadcast, newest first.
//
// Unlike GetNotifications (MetaService), this is not filtered by target: an
// administrator manages the whole history, not just the ones aimed at them.
func (a *API) ListNotificationsAdmin(ctx context.Context, req *emptypb.Empty) (*protobuf.ListNotificationsAdminResponse, error) {
	notifications, err := a.dao.NotificationsDao.GetAllNotifications()
	if err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}

	out := make([]*protobuf.AdminNotification, 0, len(notifications))
	for _, notification := range notifications {
		out = append(out, notificationMessage(notification))
	}

	return &protobuf.ListNotificationsAdminResponse{Notifications: out}, nil
}

// CreateNotification broadcasts a notification to users matching its target group.
func (a *API) CreateNotification(ctx context.Context, req *protobuf.CreateNotificationRequest) (*protobuf.AdminNotification, error) {
	target := model.NotificationTarget(req.GetTarget())
	if target < model.TargetAll || target > model.TargetAdmin {
		return nil, e.WithStatus(http.StatusBadRequest, errors.New("invalid notification target"))
	}

	notification := model.Notification{
		Body:   req.GetBody(),
		Target: target,
	}
	if title := req.GetTitle(); title != "" {
		notification.Title = &title
	}

	if err := a.dao.NotificationsDao.AddNotification(&notification); err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}

	// gorm only runs AfterFind on a query, not on the Create above, so SanitizedBody
	// is still empty; run it explicitly so the response matches what a later list
	// call would return instead of leaving the body unrendered until the next fetch.
	_ = notification.AfterFind(nil)

	return notificationMessage(notification), nil
}

// DeleteNotification deletes a notification; it stops showing to users immediately.
func (a *API) DeleteNotification(ctx context.Context, req *protobuf.DeleteNotificationRequest) (*emptypb.Empty, error) {
	if err := a.dao.NotificationsDao.DeleteNotification(uint(req.GetId())); err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}
	return &emptypb.Empty{}, nil
}

func notificationMessage(notification model.Notification) *protobuf.AdminNotification {
	var title string
	if notification.Title != nil {
		title = *notification.Title
	}

	return &protobuf.AdminNotification{
		Id:        uint32(notification.ID),
		Title:     title,
		Body:      notification.SanitizedBody,
		Target:    uint32(notification.Target),
		CreatedAt: timestamppb.New(notification.CreatedAt),
	}
}
