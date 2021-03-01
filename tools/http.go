package tools

import (
	"TUM-Live/dao"
	"TUM-Live/model"
	"context"
	"errors"
	"net/http"
	"time"
)

func GetUser(w http.ResponseWriter, r *http.Request, user *model.User) (err error) {
	sid, err := GetSID(r)
	if err != nil {
		return err
	}
	foundUser, err := dao.GetUserBySID(context.Background(), sid)
	if err != nil {
		// delete invalid session cookie
		http.SetCookie(w, &http.Cookie{Name: "SID", Expires: time.Now().AddDate(0, 0, -1)})
		return err
	}
	*user = foundUser
	return nil
}

func GetSID(r *http.Request) (SID string, err error) {
	cookie, err := r.Cookie("SID")
	if err != nil {
		return "", errors.New("no session cookie")
	}
	return cookie.Value, nil
}

func RequirePermission(w http.ResponseWriter, r http.Request, permLevel int) (user *model.User) {
	sid, err := GetSID(&r)
	if err != nil {
		w.WriteHeader(http.StatusForbidden)
		return nil
	}
	foundUser, err := dao.GetUserBySID(context.Background(), sid)
	if err != nil { // no record for session id
		w.WriteHeader(http.StatusForbidden)
		// delete invalid session cookie
		http.SetCookie(w, &http.Cookie{Name: "SID", Expires: time.Now().AddDate(0, 0, -1)})
		return nil
	}
	if foundUser.Role > permLevel {
		w.WriteHeader(http.StatusForbidden)
		return nil
	}

	return &foundUser
}
