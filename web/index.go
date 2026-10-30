package web

import (
	"context"

	"github.com/TUM-Dev/gocast/dao"
)

var VersionTag string

// IsFreshInstallation Checks whether there are users in the database and
// returns true if so, false if not.
func IsFreshInstallation(ctx context.Context, usersDao dao.UsersDao) (bool, error) {
	res, err := usersDao.AreUsersEmpty(ctx) // fresh installation?
	if err != nil {
		return false, err
	} else if res {
		return true, nil
	}

	return false, nil
}
