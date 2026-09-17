package apiv2

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	e "github.com/TUM-Dev/gocast/apiv2/errors"
	protobuf "github.com/TUM-Dev/gocast/apiv2/protobuf/server"
	"github.com/TUM-Dev/gocast/model"
)

// Every RPC here is gated on PermAdministerServer by its policy in services.go.

// ListServerNotificationsAdmin lists every server notification, past, active and
// future, for editing. GetServerNotifications (MetaService, public) filters this down
// to what is currently active; the admin page needs the rest too.
func (a *API) ListServerNotificationsAdmin(ctx context.Context, req *emptypb.Empty) (*protobuf.ListServerNotificationsAdminResponse, error) {
	notifications, err := a.dao.ServerNotificationDao.GetAllServerNotifications()
	if err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}

	out := make([]*protobuf.ServerNotificationAdmin, 0, len(notifications))
	for _, notification := range notifications {
		out = append(out, serverNotificationAdminMessage(notification))
	}

	return &protobuf.ListServerNotificationsAdminResponse{Notifications: out}, nil
}

// CreateServerNotification creates a banner shown site-wide between start and expires.
func (a *API) CreateServerNotification(ctx context.Context, req *protobuf.CreateServerNotificationRequest) (*protobuf.ServerNotificationAdmin, error) {
	if req.GetText() == "" {
		return nil, e.WithStatus(http.StatusBadRequest, errors.New("a message is required"))
	}

	notification := model.ServerNotification{
		Text:    req.GetText(),
		Warn:    req.GetWarn(),
		Start:   req.GetStart().AsTime(),
		Expires: req.GetExpires().AsTime(),
	}
	// BeforeCreate also rejects this, but only once the query has been built;
	// checking here surfaces it as a client error rather than a generic 500.
	if notification.Expires.Before(notification.Start) {
		return nil, e.WithStatus(http.StatusBadRequest, errors.New("expires must not be before start"))
	}

	if err := a.dao.ServerNotificationDao.CreateServerNotification(&notification); err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}

	return serverNotificationAdminMessage(notification), nil
}

// UpdateServerNotification replaces a server notification's text, severity and active
// window.
func (a *API) UpdateServerNotification(ctx context.Context, req *protobuf.UpdateServerNotificationRequest) (*protobuf.ServerNotificationAdmin, error) {
	if req.GetText() == "" {
		return nil, e.WithStatus(http.StatusBadRequest, errors.New("a message is required"))
	}

	start := req.GetStart().AsTime()
	expires := req.GetExpires().AsTime()
	if expires.Before(start) {
		return nil, e.WithStatus(http.StatusBadRequest, errors.New("expires must not be before start"))
	}

	id := req.GetId()
	notification := model.ServerNotification{
		Text:    req.GetText(),
		Warn:    req.GetWarn(),
		Start:   start,
		Expires: expires,
	}
	if err := a.dao.ServerNotificationDao.UpdateServerNotification(notification, strconv.FormatUint(uint64(id), 10)); err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}

	notification.ID = uint(id)
	return serverNotificationAdminMessage(notification), nil
}

// DeleteServerNotification deletes a server notification; it stops showing
// immediately.
func (a *API) DeleteServerNotification(ctx context.Context, req *protobuf.DeleteServerNotificationRequest) (*emptypb.Empty, error) {
	id := strconv.FormatUint(uint64(req.GetId()), 10)
	if err := a.dao.ServerNotificationDao.DeleteServerNotification(id); err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}
	return &emptypb.Empty{}, nil
}

func serverNotificationAdminMessage(notification model.ServerNotification) *protobuf.ServerNotificationAdmin {
	return &protobuf.ServerNotificationAdmin{
		Id:      uint32(notification.ID),
		Text:    notification.Text,
		Warn:    notification.Warn,
		Start:   timestamppb.New(notification.Start),
		Expires: timestamppb.New(notification.Expires),
	}
}
