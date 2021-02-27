package tools

import (
	"TUM-Live/dao"
	"TUM-Live/model"
	"context"
	"errors"
	"net/http"
)

var ErrorNotLoggedIn = errors.New("not logged in")
var genericError = errors.New("something went wrong")

func GetUser(r *http.Request, user *model.User) (err error) {
	sid, err := GetSID(r)
	if err != nil {
		return ErrorNotLoggedIn
	}
	foundUser, err := dao.GetUserBySID(context.Background(), sid)
	if err != nil {
		// Session id invalid.
		return ErrorNotLoggedIn
	}
	*user = foundUser
	return nil
}

func GetSID(r *http.Request) (SID string, err error) {
	cookie, err := r.Cookie("SID")
	if err != nil {
		return "", ErrorNotLoggedIn
	}
	return cookie.Value, nil
}
