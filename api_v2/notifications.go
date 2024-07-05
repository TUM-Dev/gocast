// Package api_v2 provides API endpoints for the application.
package api_v2

import (
	"context"
	"errors"
	"net/http"

	e "github.com/TUM-Dev/gocast/api_v2/errors"
	h "github.com/TUM-Dev/gocast/api_v2/helpers"
	"github.com/TUM-Dev/gocast/api_v2/protobuf"
	s "github.com/TUM-Dev/gocast/api_v2/services"
)

func (a *API) GetBannerAlerts(ctx context.Context, req *protobuf.GetBannerAlertsRequest) (*protobuf.GetBannerAlertsResponse, error) {
	a.log.Info("GetBannerAlerts")
	alerts, err := s.FetchBannerAlerts(a.db)
	if err != nil {
		return nil, err
	}

	resp := make([]*protobuf.BannerAlert, len(alerts))
	for i, alert := range alerts {
		resp[i] = h.ParseBannerAlertToProto(alert)
	}

	return &protobuf.GetBannerAlertsResponse{
		BannerAlerts: resp,
	}, nil
}

func (a *API) GetFeatureNotifications(ctx context.Context, req *protobuf.GetFeatureNotificationsRequest) (*protobuf.GetFeatureNotificationsResponse, error) {
	a.log.Info("GetUserNotifications")
	u, err := a.getCurrent(ctx)
	if err != nil {
		return nil, e.WithStatus(http.StatusUnauthorized, err)
	}

	notifications, err := s.FetchUserNotifications(a.db, u)
	if err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}

	resp := make([]*protobuf.FeatureNotification, len(notifications))

	for i, notification := range notifications {
		resp[i] = h.ParseFeatureNotificationToProto(notification)
	}

	return &protobuf.GetFeatureNotificationsResponse{
		FeatureNotifications: resp,
	}, nil
}

func (a *API) PostDeviceToken(ctx context.Context, req *protobuf.PostDeviceTokenRequest) (*protobuf.PostDeviceTokenResponse, error) {
	a.log.Info("PostDeviceToken")

	if req.DeviceToken == "" {
		return nil, e.WithStatus(http.StatusBadRequest, errors.New("device_token must not be empty"))
	}

	u, err := a.getCurrent(ctx)
	if err != nil {
		return nil, e.WithStatus(http.StatusUnauthorized, err)
	}

	if err = s.PostDeviceToken(a.db, *u, req.DeviceToken); err != nil {
		return nil, err
	}

	return &protobuf.PostDeviceTokenResponse{}, nil
}

func (a *API) DeleteDeviceToken(ctx context.Context, req *protobuf.DeleteDeviceTokenRequest) (*protobuf.DeleteDeviceTokenResponse, error) {
	a.log.Info("DeleteDeviceToken")

	if req.DeviceToken == "" {
		return nil, e.WithStatus(http.StatusBadRequest, errors.New("device_token must not be empty"))
	}

	uID, err := a.getCurrentID(ctx)
	if err != nil {
		return nil, e.WithStatus(http.StatusUnauthorized, err)
	}

	if err = s.DeleteDeviceToken(a.db, uID, req.DeviceToken); err != nil {
		return nil, err
	}

	return &protobuf.DeleteDeviceTokenResponse{}, nil
}
