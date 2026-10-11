package apiv2

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"

	e "github.com/TUM-Dev/gocast/apiv2/errors"
	"github.com/TUM-Dev/gocast/model"
	"github.com/TUM-Dev/gocast/tools"
)

// impersonateRouteOf matches POST /api/v2/users/{id}/impersonate and returns the id.
func impersonateRouteOf(r *http.Request) (uint, bool) {
	if r.Method != http.MethodPost {
		return 0, false
	}
	rest, ok := strings.CutPrefix(r.URL.Path, "/api/v2/users/")
	if !ok {
		return 0, false
	}
	idStr, ok := strings.CutSuffix(rest, "/impersonate")
	if !ok {
		return 0, false
	}
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil || id == 0 {
		return 0, false
	}
	return uint(id), true
}

// handleImpersonate replaces v1's POST /api/users/impersonate. It signs the caller in
// as another account by replacing their session cookie, which is why it sits beside the
// gateway: an RPC cannot set one.
//
// Who may: whoever holds PermManageUsers, as in v1. Nothing is remembered about the
// original account, so, as in v1, there is no way back but to sign in again; the SPA
// says so before asking. v1 wrote no audit entry either; this logs the pair instead.
func (a *API) handleImpersonate(c *gin.Context, targetID uint) {
	ctx := incomingContext(c.Request)

	caller, err := a.getCurrent(ctx)
	if err != nil {
		writeUploadError(c, e.WithStatus(http.StatusUnauthorized, errors.New("sign in first")))
		return
	}
	if !caller.Can(model.PermManageUsers) {
		writeUploadError(c, status.Error(codes.PermissionDenied, "not allowed to sign in as another user"))
		return
	}

	target, err := a.dao.UsersDao.GetUserByID(ctx, targetID)
	if err != nil || target.ID == 0 {
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			a.log.Error("could not look up the user to impersonate", "err", err, "user", targetID)
			writeUploadError(c, e.WithStatus(http.StatusInternalServerError, errors.New("could not look up the user")))
			return
		}
		writeUploadError(c, e.WithStatus(http.StatusNotFound, errors.New("user not found")))
		return
	}

	a.log.Info("impersonation", "admin", caller.ID, "as", target.ID)
	tools.StartSession(c, &tools.SessionData{Userid: target.ID})
	// StartSession only logs when it cannot sign the cookie; answering success with no
	// cookie would leave the admin believing they are someone else.
	if c.Writer.Header().Get("Set-Cookie") == "" {
		writeUploadError(c, e.WithStatus(http.StatusInternalServerError, errors.New("could not start a session")))
		return
	}

	// The cookie is the response; it must not be replayed from a cache.
	c.Header("Cache-Control", "no-store")
	c.Status(http.StatusNoContent)
}
