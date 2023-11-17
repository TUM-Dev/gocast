package api_v2

import (
	"context"
	"errors"
	"github.com/TUM-Dev/gocast/api_v2/e"
	"github.com/TUM-Dev/gocast/api_v2/protobuf"
	"github.com/TUM-Dev/gocast/model"
	"gorm.io/gorm"
	"net/http"
	"strings"
)

// todo implement this functionality
func (a *API) PasswordAuth(ctx context.Context, req *protobuf.PasswordAuthRequest) (*protobuf.PasswordAuthResponse, error) {
	return &protobuf.PasswordAuthResponse{
		AuthToken: "abc",
	}, nil
}

// todo: this can be removed and serves only as an example
func (a *API) GetNumberOfUsers(ctx context.Context, req *protobuf.NumberOfUsersRequest) (*protobuf.NumberOfUsersResponse, error) {
	u, err := a.getUser(ctx)
	if err != nil {
		return nil, e.WithStatus(http.StatusUnauthorized, err)
	}
	if !strings.Contains(u.Name, "admin") {
		return nil, e.WithStatus(http.StatusForbidden, errors.New("only admins can access this endpoint"))
	}

	a.log.Info("GetNumberOfUsers")
	var numberOfUsers int64
	err = a.db.Model(&model.User{}).Count(&numberOfUsers).Error
	if err != nil && errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, e.WithStatus(http.StatusNotFound, err)
	} else if err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}
	return &protobuf.NumberOfUsersResponse{
		NumberOfUsers: int32(numberOfUsers),
	}, nil
}
