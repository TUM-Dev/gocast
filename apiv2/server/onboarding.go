package apiv2

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strings"

	"google.golang.org/protobuf/types/known/emptypb"

	e "github.com/TUM-Dev/gocast/apiv2/errors"
	protobuf "github.com/TUM-Dev/gocast/apiv2/protobuf/server"
	"github.com/TUM-Dev/gocast/model"
)

// CreateFirstUser creates the administrator of a fresh deployment, the one account
// that can exist before anyone can sign in. It serves anonymous callers for exactly
// as long as the users table is empty; getFrontendConfig reports that same state.
func (a *API) CreateFirstUser(ctx context.Context, req *protobuf.CreateFirstUserRequest) (*emptypb.Empty, error) {
	fresh, err := a.dao.UsersDao.AreUsersEmpty(ctx)
	if err != nil {
		a.log.Error("can't tell whether the deployment has users", "err", err)
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}
	if !fresh {
		return nil, e.WithStatus(http.StatusForbidden, errors.New("the deployment already has users; an administrator creates further ones"))
	}

	name := strings.TrimSpace(req.GetName())
	email := strings.TrimSpace(req.GetEmail())
	switch {
	case name == "":
		return nil, e.WithStatus(http.StatusBadRequest, errors.New("name is required"))
	case email == "" || !strings.Contains(email, "@"):
		return nil, e.WithStatus(http.StatusBadRequest, errors.New("a valid email address is required"))
	}
	user := model.User{
		Name:  name,
		Email: sql.NullString{String: email, Valid: true},
		Role:  model.AdminType,
	}
	if err := user.SetPassword(req.GetPassword()); err != nil {
		return nil, e.WithStatus(http.StatusBadRequest, errors.New("the password must be at least 8 characters long"))
	}
	if err := a.dao.UsersDao.CreateUser(ctx, &user); err != nil {
		a.log.Error("can't create the first user", "err", err)
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}
	return &emptypb.Empty{}, nil
}
