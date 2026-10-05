package apiv2

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"google.golang.org/protobuf/types/known/emptypb"

	e "github.com/TUM-Dev/gocast/apiv2/errors"
	protobuf "github.com/TUM-Dev/gocast/apiv2/protobuf/server"
	"github.com/TUM-Dev/gocast/model"
)

// The reset key is the credential, mailed by resetPassword or an account invite;
// both RPCs serve anonymous callers.

func (a *API) userByResetKey(key string) (model.User, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return model.User{}, e.WithStatus(http.StatusBadRequest, errors.New("key is required"))
	}
	user, err := a.dao.UsersDao.GetUserByResetKey(key)
	if err != nil {
		return model.User{}, e.WithStatus(http.StatusNotFound, errors.New("this link is not valid any more"))
	}
	return user, nil
}

// CheckPasswordResetKey says whether a key still opens the set-password page; the
// page asks before showing the form, as the template redirected home instead.
func (a *API) CheckPasswordResetKey(ctx context.Context, req *protobuf.PasswordResetKeyRequest) (*emptypb.Empty, error) {
	if _, err := a.userByResetKey(req.GetKey()); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

// SetPasswordByResetKey sets the password of the user the key was mailed to and
// spends the key.
func (a *API) SetPasswordByResetKey(ctx context.Context, req *protobuf.SetPasswordByResetKeyRequest) (*emptypb.Empty, error) {
	user, err := a.userByResetKey(req.GetKey())
	if err != nil {
		return nil, err
	}
	if err := user.SetPassword(req.GetPassword()); err != nil {
		// The only way SetPassword fails on input is the length rule.
		return nil, e.WithStatus(http.StatusBadRequest, errors.New("the password must be at least 8 characters long"))
	}
	if err := a.dao.UsersDao.UpdateUser(user); err != nil {
		a.log.Error("can't save the new password", "err", err, "user", user.ID)
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}
	a.dao.UsersDao.DeleteResetKey(strings.TrimSpace(req.GetKey()))
	return &emptypb.Empty{}, nil
}
