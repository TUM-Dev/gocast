package apiv2

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"testing"

	"go.uber.org/mock/gomock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"gorm.io/gorm"

	protobuf "github.com/TUM-Dev/gocast/apiv2/protobuf/server"
	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/mock_dao"
	"github.com/TUM-Dev/gocast/model"
)

func user(id uint, name, email string, role uint) model.User {
	return model.User{
		Model: gorm.Model{ID: id},
		Name:  name,
		Email: sql.NullString{String: email, Valid: email != ""},
		Role:  role,
	}
}

// Which of the two listings masks is the whole reason they are separate endpoints,
// and getting it backwards is not visible in a type.
func TestListStaffDoesNotMask(t *testing.T) {
	ctrl := gomock.NewController(t)
	usersMock := mock_dao.NewMockUsersDao(ctrl)
	usersMock.EXPECT().GetAllAdminsAndLecturers(gomock.Any()).
		DoAndReturn(func(users *[]model.User) error {
			*users = []model.User{user(1, "Anja Admin", "anja@example.org", model.AdminType)}
			return nil
		}).Times(1)

	api := &API{dao: dao.DaoWrapper{UsersDao: usersMock}, log: slog.Default()}

	resp, err := api.ListStaff(context.Background(), &emptypb.Empty{})
	if err != nil {
		t.Fatalf("ListStaff: %v", err)
	}

	if got := resp.Users[0].Email; got != "anja@example.org" {
		t.Errorf("email = %q, want it unmasked", got)
	}
}

func TestSearchUsersMasks(t *testing.T) {
	ctrl := gomock.NewController(t)
	usersMock := mock_dao.NewMockUsersDao(ctrl)
	found := user(2, "Stephanie Studi", "stephanie@example.org", model.StudentType)
	found.LrzID = "ab12cde"
	usersMock.EXPECT().SearchUser(gomock.Any()).Return([]model.User{found}, nil).Times(1)

	api := &API{dao: dao.DaoWrapper{UsersDao: usersMock}, log: slog.Default()}

	resp, err := api.SearchUsers(context.Background(), &protobuf.SearchUsersRequest{Query: "stephanie"})
	if err != nil {
		t.Fatalf("SearchUsers: %v", err)
	}

	if got := resp.Users[0].Email; got != "s********@example.org" {
		t.Errorf("email = %q, want it masked", got)
	}
	if got := resp.Users[0].LrzId; got != "ab**cde" {
		t.Errorf("lrz id = %q, want its digits masked", got)
	}
	// The name is what the list is read by, so it is not masked.
	if got := resp.Users[0].Name; got != "Stephanie Studi" {
		t.Errorf("name = %q, want it unmasked", got)
	}
}

// An address the masker cannot parse must not be passed through as it is.
func TestSearchUsersDropsAnAddressItCannotMask(t *testing.T) {
	ctrl := gomock.NewController(t)
	usersMock := mock_dao.NewMockUsersDao(ctrl)
	usersMock.EXPECT().SearchUser(gomock.Any()).
		Return([]model.User{user(2, "No Address", "not-an-email", model.StudentType)}, nil).Times(1)

	api := &API{dao: dao.DaoWrapper{UsersDao: usersMock}, log: slog.Default()}

	resp, err := api.SearchUsers(context.Background(), &protobuf.SearchUsersRequest{Query: "address"})
	if err != nil {
		t.Fatalf("SearchUsers: %v", err)
	}

	if got := resp.Users[0].Email; got != "" {
		t.Errorf("email = %q, want it dropped rather than sent unmasked", got)
	}
}

func TestSearchUsersQueryLength(t *testing.T) {
	role := uint32(model.LecturerType)

	tests := []struct {
		name     string
		req      *protobuf.SearchUsersRequest
		wantCode codes.Code
		// Which dao call the request should reach, if any.
		expectSearch   bool
		expectWithRole bool
	}{
		{
			name:         "a long enough query searches",
			req:          &protobuf.SearchUsersRequest{Query: "prof"},
			expectSearch: true,
		},
		{
			// Otherwise an empty box asks for every account.
			name:     "a short query with no role is refused",
			req:      &protobuf.SearchUsersRequest{Query: "pr"},
			wantCode: codes.InvalidArgument,
		},
		{
			// "Show me every lecturer" is a legitimate search with no query at all.
			name:           "a role with no query searches by role",
			req:            &protobuf.SearchUsersRequest{Role: &role},
			expectWithRole: true,
		},
		{
			name:     "a role that does not exist is refused",
			req:      &protobuf.SearchUsersRequest{Role: func() *uint32 { r := uint32(99); return &r }()},
			wantCode: codes.InvalidArgument,
		},
		{
			// Stripped before the length is judged, leaving nothing to search on.
			name:     "a query of only punctuation is refused",
			req:      &protobuf.SearchUsersRequest{Query: "%%%%"},
			wantCode: codes.InvalidArgument,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			usersMock := mock_dao.NewMockUsersDao(ctrl)

			search := usersMock.EXPECT().SearchUser(gomock.Any()).Return(nil, nil)
			withRole := usersMock.EXPECT().SearchUserWithRole(gomock.Any(), gomock.Any()).Return(nil, nil)
			if tt.expectSearch {
				search.Times(1)
			} else {
				search.Times(0)
			}
			if tt.expectWithRole {
				withRole.Times(1)
			} else {
				withRole.Times(0)
			}

			api := &API{dao: dao.DaoWrapper{UsersDao: usersMock}, log: slog.Default()}

			_, err := api.SearchUsers(context.Background(), tt.req)
			if got := status.Code(err); got != tt.wantCode {
				t.Errorf("code = %v, want %v", got, tt.wantCode)
			}
		})
	}
}

// The dao reports no matches as a missing record, which is not a 404.
func TestSearchUsersMatchingNobody(t *testing.T) {
	ctrl := gomock.NewController(t)
	usersMock := mock_dao.NewMockUsersDao(ctrl)
	usersMock.EXPECT().SearchUser(gomock.Any()).Return(nil, gorm.ErrRecordNotFound).Times(1)

	api := &API{dao: dao.DaoWrapper{UsersDao: usersMock}, log: slog.Default()}

	resp, err := api.SearchUsers(context.Background(), &protobuf.SearchUsersRequest{Query: "nobody"})
	if err != nil {
		t.Fatalf("SearchUsers: %v", err)
	}
	if len(resp.Users) != 0 {
		t.Errorf("got %d users, want none", len(resp.Users))
	}
}

func TestUpdateUserRole(t *testing.T) {
	const callerID, targetID = 1, 2
	signedIn := user(callerID, "Anja Admin", "anja@example.org", model.AdminType)

	tests := []struct {
		name       string
		userID     uint32
		role       uint32
		wantCode   codes.Code
		wantUpdate bool
	}{
		{
			name:       "promotes another account",
			userID:     targetID,
			role:       model.LecturerType,
			wantUpdate: true,
		},
		{
			name:     "refuses a role that does not exist",
			userID:   targetID,
			role:     99,
			wantCode: codes.InvalidArgument,
		},
		{
			// Removes the permission that allowed it; v1 allowed this.
			name:     "refuses to change the caller's own role",
			userID:   callerID,
			role:     model.StudentType,
			wantCode: codes.InvalidArgument,
		},
		{
			// Changes nothing, so it should not trip the self-demotion guard.
			name:       "allows the caller to set their own role to what it already is",
			userID:     callerID,
			role:       model.AdminType,
			wantUpdate: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			usersMock := mock_dao.NewMockUsersDao(ctrl)
			usersMock.EXPECT().GetUserByID(gomock.Any(), gomock.Any()).
				DoAndReturn(func(_ context.Context, id uint) (model.User, error) {
					if id == callerID {
						return signedIn, nil
					}
					return user(id, "Someone", "someone@example.org", model.StudentType), nil
				}).AnyTimes()

			update := usersMock.EXPECT().UpdateUser(gomock.Any()).Return(nil)
			if tt.wantUpdate {
				update.Times(1)
			} else {
				update.Times(0)
			}

			api := &API{dao: dao.DaoWrapper{UsersDao: usersMock}, log: slog.Default()}
			ctx := context.WithValue(context.Background(), callerKey{}, &caller{user: &signedIn})

			_, err := api.UpdateUserRole(ctx, &protobuf.UpdateUserRoleRequest{
				UserId: tt.userID,
				Role:   tt.role,
			})

			if got := status.Code(err); got != tt.wantCode {
				t.Errorf("code = %v, want %v", got, tt.wantCode)
			}
		})
	}
}

// Otherwise this page can remove everyone able to undo it.
func TestDeleteUserRefusesAdministrators(t *testing.T) {
	ctrl := gomock.NewController(t)
	usersMock := mock_dao.NewMockUsersDao(ctrl)
	usersMock.EXPECT().IsUserAdmin(gomock.Any(), uint(1)).Return(true, nil).Times(1)
	usersMock.EXPECT().DeleteUser(gomock.Any(), gomock.Any()).Times(0)

	api := &API{dao: dao.DaoWrapper{UsersDao: usersMock}, log: slog.Default()}

	_, err := api.DeleteUser(context.Background(), &protobuf.DeleteUserRequest{UserId: 1})

	if got := status.Code(err); got != codes.InvalidArgument {
		t.Errorf("code = %v, want %v", got, codes.InvalidArgument)
	}
}

func TestDeleteUser(t *testing.T) {
	ctrl := gomock.NewController(t)
	usersMock := mock_dao.NewMockUsersDao(ctrl)
	usersMock.EXPECT().IsUserAdmin(gomock.Any(), uint(2)).Return(false, nil).Times(1)
	usersMock.EXPECT().DeleteUser(gomock.Any(), uint(2)).Return(nil).Times(1)

	api := &API{dao: dao.DaoWrapper{UsersDao: usersMock}, log: slog.Default()}

	if _, err := api.DeleteUser(context.Background(), &protobuf.DeleteUserRequest{UserId: 2}); err != nil {
		t.Fatalf("DeleteUser: %v", err)
	}
}

func TestCreateUserRequiresANameAndAnEmail(t *testing.T) {
	tests := []struct {
		name string
		req  *protobuf.CreateUserRequest
	}{
		{name: "no name", req: &protobuf.CreateUserRequest{Email: "someone@example.org"}},
		{name: "no email", req: &protobuf.CreateUserRequest{Name: "Someone"}},
		{name: "blank name", req: &protobuf.CreateUserRequest{Name: "   ", Email: "s@example.org"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			usersMock := mock_dao.NewMockUsersDao(ctrl)
			usersMock.EXPECT().CreateUser(gomock.Any(), gomock.Any()).Times(0)

			api := &API{dao: dao.DaoWrapper{UsersDao: usersMock}, log: slog.Default()}

			_, err := api.CreateUser(context.Background(), tt.req)
			if got := status.Code(err); got != codes.InvalidArgument {
				t.Errorf("code = %v, want %v", got, codes.InvalidArgument)
			}
		})
	}
}

// Created as a lecturer, never an administrator.
func TestCreateUserMakesALecturer(t *testing.T) {
	ctrl := gomock.NewController(t)
	usersMock := mock_dao.NewMockUsersDao(ctrl)
	emailMock := mock_dao.NewMockEmailDao(ctrl)

	usersMock.EXPECT().AreUsersEmpty(gomock.Any()).Return(false, nil).Times(1)
	// Asked before creating, then again by the invitation.
	usersMock.EXPECT().GetUserByEmail(gomock.Any(), "new@example.org").
		Return(model.User{}, gorm.ErrRecordNotFound).Times(1)

	var created model.User
	usersMock.EXPECT().CreateUser(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, u *model.User) error {
			u.Model.ID = 7
			created = *u
			return nil
		}).Times(1)

	// The only thing that lets the new account sign in.
	usersMock.EXPECT().GetUserByEmail(gomock.Any(), "new@example.org").
		DoAndReturn(func(_ context.Context, _ string) (model.User, error) { return created, nil }).Times(1)
	usersMock.EXPECT().CreateRegisterLink(gomock.Any(), gomock.Any()).
		Return(model.RegisterLink{RegisterSecret: "secret"}, nil).Times(1)
	emailMock.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil).Times(1)

	api := &API{
		dao: dao.DaoWrapper{UsersDao: usersMock, EmailDao: emailMock},
		log: slog.Default(),
	}

	resp, err := api.CreateUser(context.Background(), &protobuf.CreateUserRequest{
		Name:  "New Lecturer",
		Email: "new@example.org",
	})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	if created.Role != model.LecturerType {
		t.Errorf("created with role %d, want %d", created.Role, model.LecturerType)
	}
	if resp.Id != 7 {
		t.Errorf("id = %d, want the id the database assigned", resp.Id)
	}
}

// The account exists but nobody can sign in to it, so this must not be swallowed.
func TestCreateUserReportsAnInvitationItCouldNotSend(t *testing.T) {
	ctrl := gomock.NewController(t)
	usersMock := mock_dao.NewMockUsersDao(ctrl)

	usersMock.EXPECT().AreUsersEmpty(gomock.Any()).Return(false, nil).Times(1)
	usersMock.EXPECT().CreateUser(gomock.Any(), gomock.Any()).Return(nil).Times(1)
	// Not found on the way in, then unreachable when the invitation looks again.
	usersMock.EXPECT().GetUserByEmail(gomock.Any(), gomock.Any()).
		Return(model.User{}, gorm.ErrRecordNotFound).Times(1)
	usersMock.EXPECT().GetUserByEmail(gomock.Any(), gomock.Any()).
		Return(model.User{}, errors.New("gone")).Times(1)

	api := &API{dao: dao.DaoWrapper{UsersDao: usersMock}, log: slog.Default()}

	_, err := api.CreateUser(context.Background(), &protobuf.CreateUserRequest{
		Name:  "New Lecturer",
		Email: "new@example.org",
	})
	if err == nil {
		t.Fatal("an invitation that could not be sent was reported as success")
	}
}

// The email column is unique; this used to surface as a constraint violation.
func TestCreateUserRefusesAnAddressThatAlreadyHasAnAccount(t *testing.T) {
	ctrl := gomock.NewController(t)
	usersMock := mock_dao.NewMockUsersDao(ctrl)

	usersMock.EXPECT().AreUsersEmpty(gomock.Any()).Return(false, nil).Times(1)
	usersMock.EXPECT().GetUserByEmail(gomock.Any(), "taken@example.org").
		Return(user(3, "Already Here", "taken@example.org", model.LecturerType), nil).Times(1)
	usersMock.EXPECT().CreateUser(gomock.Any(), gomock.Any()).Times(0)

	api := &API{dao: dao.DaoWrapper{UsersDao: usersMock}, log: slog.Default()}

	_, err := api.CreateUser(context.Background(), &protobuf.CreateUserRequest{
		Name:  "Someone Else",
		Email: "taken@example.org",
	})

	if got := status.Code(err); got != codes.AlreadyExists {
		t.Errorf("code = %v, want %v", got, codes.AlreadyExists)
	}
}
