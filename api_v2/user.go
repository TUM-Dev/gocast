package api_v2

import (
	"context"
	"github.com/TUM-Dev/gocast/api_v2/e"
	"github.com/TUM-Dev/gocast/api_v2/protobuf"
	"net/http"
)

func (a *API) GetUser(ctx context.Context, req *protobuf.GetUserRequest) (*protobuf.GetUserResponse, error) {
	a.log.Info("GetUser")
	u, err := a.getCurrent(ctx)
	if err != nil {
		return nil, e.WithStatus(http.StatusUnauthorized, err)
	}

	response := &protobuf.GetUserResponse{
		User: &protobuf.User{
			Id:                 uint32(u.ID),
			Name:               u.Name,
			Email:              u.Email.String,
			MatriculationNumber: u.MatriculationNumber,
			LrzID:              u.LrzID,
			Role:               uint32(u.Role),
			Settings:           []*protobuf.UserSetting{},
		},
	}

	if u.LastName != nil {
		response.User.LastName = *u.LastName
	}

	for _, setting := range u.Settings {
		response.User.Settings = append(response.User.Settings, &protobuf.UserSetting{
			Id:       uint32(setting.ID),
			UserID:   uint32(setting.UserID),
			Type:     protobuf.UserSettingType(setting.Type),
			Value:    setting.Value,
		})
	}

	return response, nil
}
