package apiv2

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"

	e "github.com/TUM-Dev/gocast/apiv2/errors"
	"github.com/TUM-Dev/gocast/model"
	"github.com/TUM-Dev/gocast/tools"
)

// impersonatePath is POST /api/v2/users/{id}/impersonate.
const impersonatePath = "POST /api/v2/users/{id}/impersonate"

// impersonateIDOf reads the route's id. The pattern matches any text there, so a
// request whose id is not a number is not this endpoint's and goes to the gateway.
func impersonateIDOf(r *http.Request) (uint, bool) {
	id, err := strconv.ParseUint(r.PathValue("id"), 10, 32)
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
// says so before asking. v1 wrote no audit entry; this writes one (AuditImpersonation) naming both accounts.
func (a *API) handleImpersonate(w http.ResponseWriter, r *http.Request, targetID uint) {
	ctx := incomingContext(r)

	caller, err := a.getCurrent(ctx)
	if err != nil {
		writeUploadError(w, e.WithStatus(http.StatusUnauthorized, errors.New("sign in first")))
		return
	}
	if !caller.Can(model.PermManageUsers) {
		writeUploadError(w, status.Error(codes.PermissionDenied, "not allowed to sign in as another user"))
		return
	}

	target, err := a.dao.UsersDao.GetUserByID(ctx, targetID)
	if err != nil || target.ID == 0 {
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			a.log.Error("could not look up the user to impersonate", "err", err, "user", targetID)
			writeUploadError(w, e.WithStatus(http.StatusInternalServerError, errors.New("could not look up the user")))
			return
		}
		writeUploadError(w, e.WithStatus(http.StatusNotFound, errors.New("user not found")))
		return
	}

	a.log.Info("impersonation", "admin", caller.ID, "as", target.ID)
	// The actor is the audit entry's user (a.audit reads it from ctx, which is the
	// caller); the target is named in the message. Written before the session changes,
	// so a signed-in impersonation never goes unrecorded.
	a.audit(ctx, model.AuditImpersonation, fmt.Sprintf("%s (%d) signed in as %s (%d)",
		caller.GetLoginString(), caller.ID, target.GetLoginString(), target.ID))
	tools.StartSession(w, &tools.SessionData{Userid: target.ID})
	// StartSession only logs when it cannot sign the cookie; answering success with no
	// cookie would leave the admin believing they are someone else.
	if w.Header().Get("Set-Cookie") == "" {
		writeUploadError(w, e.WithStatus(http.StatusInternalServerError, errors.New("could not start a session")))
		return
	}

	// The cookie is the response; it must not be replayed from a cache.
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusNoContent)
}
