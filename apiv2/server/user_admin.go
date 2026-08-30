package apiv2

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"regexp"
	"strings"

	"google.golang.org/protobuf/types/known/emptypb"

	e "github.com/TUM-Dev/gocast/apiv2/errors"
	protobuf "github.com/TUM-Dev/gocast/apiv2/protobuf/server"
	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/model"
	"github.com/TUM-Dev/gocast/tools"
	"gorm.io/gorm"
)

// Every RPC here is gated on PermManageUsers by its policy in services.go.

// searchQueryMinLength stops an empty box from asking for every account.
const searchQueryMinLength = 3

// The query goes into a LIKE, so strip what would mean something there.
var searchQueryAllowed = regexp.MustCompile(`[^a-zA-Z0-9 ]+`)

// assignableRoles are the roles an account may be given. v1 wrote any number it was
// sent, which could leave an account on a role that matches nothing.
var assignableRoles = map[uint]bool{
	model.AdminType:    true,
	model.LecturerType: true,
	model.GenericType:  true,
	model.StudentType:  true,
}

// ListStaff returns the administrators and lecturers, with contact details unmasked.
//
// That SearchUsers masks and this does not is inherited from the page they replace,
// and kept deliberately: what an administrator may see is a policy decision, not a port.
func (a *API) ListStaff(ctx context.Context, req *emptypb.Empty) (*protobuf.ListUsersResponse, error) {
	var users []model.User
	if err := a.dao.UsersDao.GetAllAdminsAndLecturers(&users); err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}

	return usersResponse(users, false), nil
}

// SearchUsers searches every account, with contact details masked.
func (a *API) SearchUsers(ctx context.Context, req *protobuf.SearchUsersRequest) (*protobuf.ListUsersResponse, error) {
	query := strings.TrimSpace(searchQueryAllowed.ReplaceAllString(req.GetQuery(), ""))

	// A role on its own is a valid search, so the floor applies only without one.
	if len(query) < searchQueryMinLength && req.Role == nil {
		return nil, e.WithStatus(
			http.StatusBadRequest,
			errors.New("query too short (minimum length is 3)"),
		)
	}

	var users []model.User
	var err error
	if req.Role == nil {
		users, err = a.dao.UsersDao.SearchUser(query)
	} else {
		if !assignableRoles[uint(req.GetRole())] {
			return nil, e.WithStatus(http.StatusBadRequest, errors.New("no such role"))
		}
		users, err = a.dao.UsersDao.SearchUserWithRole(query, uint64(req.GetRole()))
	}
	if err != nil {
		// The dao reports no matches as a missing record; a search that finds
		// nothing is an empty list, not a 404.
		return usersResponse(nil, true), nil
	}

	return usersResponse(users, true), nil
}

// CreateUser creates a lecturer account and emails an invitation to set a password.
func (a *API) CreateUser(ctx context.Context, req *protobuf.CreateUserRequest) (*protobuf.UserSummary, error) {
	name := strings.TrimSpace(req.GetName())
	email := strings.TrimSpace(req.GetEmail())
	if name == "" || email == "" {
		return nil, e.WithStatus(http.StatusBadRequest, errors.New("a name and an email are required"))
	}

	// An empty database means onboarding, which creates the first administrator
	// without credentials.
	empty, err := a.dao.UsersDao.AreUsersEmpty(ctx)
	if err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}
	if empty {
		return nil, e.WithStatus(
			http.StatusBadRequest,
			errors.New("no users in the database; use the onboarding page instead"),
		)
	}

	// The email column is unique, so without this the answer is a constraint
	// violation rather than "that account already exists".
	if _, err := a.dao.UsersDao.GetUserByEmail(ctx, email); err == nil {
		return nil, e.WithStatus(
			http.StatusConflict,
			errors.New("an account with that email already exists"),
		)
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}

	user := model.User{
		Name:  name,
		Email: sql.NullString{String: email, Valid: true},
		Role:  model.LecturerType,
	}
	if err := a.dao.UsersDao.CreateUser(ctx, &user); err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}

	// The account exists by now, but without the invitation nobody can sign in to
	// it — so this is reported rather than swallowed or retried by creating another.
	if err := tools.SendAccountInvite(ctx, a.dao, email); err != nil {
		a.log.Error("account created but its invitation could not be sent", "err", err, "email", email)
		return nil, e.WithStatus(
			http.StatusInternalServerError,
			errors.New("the account was created but its invitation could not be sent"),
		)
	}

	return userSummary(user, false), nil
}

// UpdateUserRole sets an account's role.
func (a *API) UpdateUserRole(ctx context.Context, req *protobuf.UpdateUserRoleRequest) (*protobuf.UserSummary, error) {
	role := uint(req.GetRole())
	if !assignableRoles[role] {
		return nil, e.WithStatus(http.StatusBadRequest, errors.New("no such role"))
	}

	caller, err := a.getCurrent(ctx)
	if err != nil {
		return nil, e.WithStatus(http.StatusUnauthorized, err)
	}

	// Demoting yourself removes the permission that allowed it, and on a
	// single-administrator deployment nothing can put it back. v1 allowed this.
	if caller.ID == uint(req.GetUserId()) && role != caller.Role {
		return nil, e.WithStatus(http.StatusBadRequest, errors.New("you cannot change your own role"))
	}

	user, err := a.dao.UsersDao.GetUserByID(ctx, uint(req.GetUserId()))
	if err != nil {
		return nil, e.FromGorm(err, "can't find user")
	}
	if user.ID == 0 {
		return nil, e.WithStatus(http.StatusNotFound, errors.New("no such user"))
	}

	user.Role = role
	if err := a.dao.UsersDao.UpdateUser(user); err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}

	// Cached for ten seconds, so without this the caller reads back a stale role.
	dao.InvalidateUserCache(uint(req.GetUserId()))

	return userSummary(user, false), nil
}

// DeleteUser deletes an account.
func (a *API) DeleteUser(ctx context.Context, req *protobuf.DeleteUserRequest) (*emptypb.Empty, error) {
	// As in v1: otherwise this page can remove everyone able to undo it.
	isAdmin, err := a.dao.UsersDao.IsUserAdmin(ctx, uint(req.GetUserId()))
	if err != nil {
		return nil, e.FromGorm(err, "can't find user")
	}
	if isAdmin {
		return nil, e.WithStatus(http.StatusBadRequest, errors.New("administrators cannot be deleted"))
	}

	if err := a.dao.UsersDao.DeleteUser(ctx, uint(req.GetUserId())); err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}

	dao.InvalidateUserCache(uint(req.GetUserId()))

	return &emptypb.Empty{}, nil
}

func usersResponse(users []model.User, mask bool) *protobuf.ListUsersResponse {
	out := make([]*protobuf.UserSummary, 0, len(users))
	for _, user := range users {
		out = append(out, userSummary(user, mask))
	}

	return &protobuf.ListUsersResponse{Users: out}
}

// userSummary converts an account, masking its contact details when asked.
func userSummary(user model.User, mask bool) *protobuf.UserSummary {
	email := user.Email.String
	lrzID := user.LrzID

	if mask {
		// An address that cannot be masked is dropped rather than sent as it is.
		masked, err := tools.MaskEmail(email)
		if err != nil {
			masked = ""
		}
		email = masked
		lrzID = tools.MaskLogin(lrzID)
	}

	return &protobuf.UserSummary{
		Id:    uint32(user.ID),
		Name:  user.GetPreferredName(),
		Email: email,
		LrzId: lrzID,
		Role:  uint32(user.Role),
	}
}
