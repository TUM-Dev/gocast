package apiv2

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"testing"
	"time"

	"go.uber.org/mock/gomock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"gorm.io/gorm"

	protobuf "github.com/TUM-Dev/gocast/apiv2/protobuf/server"
	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/mock_dao"
	"github.com/TUM-Dev/gocast/model"
	"github.com/TUM-Dev/gocast/tools"
)

func ctxAs(u model.User) context.Context {
	return context.WithValue(context.Background(), callerKey{}, &caller{user: &u})
}

// ListTokens never returns a secret -- there is no field for one on protobuf.Token --
// but it must still carry everything the page shows, including the rtmp proxy url
// bundled alongside the list rather than fetched separately.
func TestListTokens(t *testing.T) {
	tools.Cfg = tools.Config{RtmpProxyURL: "rtmp://ingest.example.org/live"}

	admin := user(1, "Anja Admin", "anja@example.org", model.AdminType)
	lastUse := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	rows := []dao.AllTokensDto{
		{
			Token:    model.Token{Model: gorm.Model{ID: 1}, Scope: model.TokenScopeLecturer, LastUse: sql.NullTime{Valid: true, Time: lastUse}},
			UserName: "Peter Prof", UserLrzID: "ab12cde",
		},
		{
			Token:    model.Token{Model: gorm.Model{ID: 2}, Scope: model.TokenScopeAdmin},
			UserMail: "anja@example.org",
		},
	}

	ctrl := gomock.NewController(t)
	tokenMock := mock_dao.NewMockTokenDao(ctrl)
	tokenMock.EXPECT().GetAllTokens(&admin).Return(rows, nil).Times(1)

	api := &API{dao: dao.DaoWrapper{TokenDao: tokenMock}, log: slog.Default()}

	resp, err := api.ListTokens(ctxAs(admin), &emptypb.Empty{})
	if err != nil {
		t.Fatalf("ListTokens: %v", err)
	}

	if resp.RtmpProxyUrl != "rtmp://ingest.example.org/live" {
		t.Errorf("RtmpProxyUrl = %q, want the configured proxy url", resp.RtmpProxyUrl)
	}
	if len(resp.Tokens) != 2 {
		t.Fatalf("got %d tokens, want 2", len(resp.Tokens))
	}

	first := resp.Tokens[0]
	if first.UserName != "Peter Prof" || first.UserLrzId != "ab12cde" {
		t.Errorf("token without an email did not carry name/LRZ id: %+v", first)
	}
	if first.LastUse == nil || !first.LastUse.AsTime().Equal(lastUse) {
		t.Errorf("LastUse = %v, want %v", first.LastUse, lastUse)
	}
	if first.Expires != nil {
		t.Errorf("Expires = %v, want nil for a token with no expiration", first.Expires)
	}

	second := resp.Tokens[1]
	if second.UserEmail != "anja@example.org" {
		t.Errorf("UserEmail = %q, want anja@example.org", second.UserEmail)
	}
}

func TestCreateTokenRefusesAnInvalidScope(t *testing.T) {
	admin := user(1, "Anja Admin", "anja@example.org", model.AdminType)

	ctrl := gomock.NewController(t)
	tokenMock := mock_dao.NewMockTokenDao(ctrl)
	tokenMock.EXPECT().AddToken(gomock.Any()).Times(0)

	api := &API{dao: dao.DaoWrapper{TokenDao: tokenMock}, log: slog.Default()}

	_, err := api.CreateToken(ctxAs(admin), &protobuf.CreateTokenRequest{Scope: "root"})

	if got := status.Code(err); got != codes.InvalidArgument {
		t.Errorf("code = %v, want %v", got, codes.InvalidArgument)
	}
}

// Every caller of this RPC already holds users.manage by policy, so in production
// this can never trigger -- but the handler makes the check anyway rather than
// trusting the policy layer alone, and that is what this asserts.
func TestCreateTokenRefusesAdminScopeWithoutPermission(t *testing.T) {
	lecturer := user(2, "Peter Prof", "peter@example.org", model.LecturerType)

	ctrl := gomock.NewController(t)
	tokenMock := mock_dao.NewMockTokenDao(ctrl)
	tokenMock.EXPECT().AddToken(gomock.Any()).Times(0)

	api := &API{dao: dao.DaoWrapper{TokenDao: tokenMock}, log: slog.Default()}

	_, err := api.CreateToken(ctxAs(lecturer), &protobuf.CreateTokenRequest{Scope: model.TokenScopeAdmin})

	if got := status.Code(err); got != codes.InvalidArgument {
		t.Errorf("code = %v, want %v", got, codes.InvalidArgument)
	}
}

// The secret in the response is the only place it is ever readable; nothing else
// about the created token is echoed back with it.
func TestCreateTokenReturnsTheSecretExactlyOnce(t *testing.T) {
	admin := user(1, "Anja Admin", "anja@example.org", model.AdminType)

	var captured model.Token
	ctrl := gomock.NewController(t)
	tokenMock := mock_dao.NewMockTokenDao(ctrl)
	tokenMock.EXPECT().AddToken(gomock.Any()).DoAndReturn(func(token model.Token) error {
		captured = token
		return nil
	}).Times(1)

	api := &API{dao: dao.DaoWrapper{TokenDao: tokenMock}, log: slog.Default()}

	resp, err := api.CreateToken(ctxAs(admin), &protobuf.CreateTokenRequest{Scope: model.TokenScopeLecturer})
	if err != nil {
		t.Fatalf("CreateToken: %v", err)
	}

	if resp.Token == "" {
		t.Fatal("CreateToken returned an empty secret")
	}
	if captured.Token != resp.Token {
		t.Errorf("stored token %q does not match the returned secret %q", captured.Token, resp.Token)
	}
	if captured.UserID != admin.ID {
		t.Errorf("UserID = %d, want %d (the caller)", captured.UserID, admin.ID)
	}
	if captured.Expires.Valid {
		t.Error("Expires.Valid = true for a request with no expiration")
	}
}

func TestCreateTokenReportsAFailedInsert(t *testing.T) {
	admin := user(1, "Anja Admin", "anja@example.org", model.AdminType)

	ctrl := gomock.NewController(t)
	tokenMock := mock_dao.NewMockTokenDao(ctrl)
	tokenMock.EXPECT().AddToken(gomock.Any()).Return(errors.New("database is on fire")).Times(1)

	api := &API{dao: dao.DaoWrapper{TokenDao: tokenMock}, log: slog.Default()}

	_, err := api.CreateToken(ctxAs(admin), &protobuf.CreateTokenRequest{Scope: model.TokenScopeLecturer})
	if err == nil {
		t.Fatal("a failed insert was reported as success")
	}
}

func TestDeleteToken(t *testing.T) {
	admin := user(1, "Anja Admin", "anja@example.org", model.AdminType)

	ctrl := gomock.NewController(t)
	tokenMock := mock_dao.NewMockTokenDao(ctrl)
	tokenMock.EXPECT().GetTokenByID("5").Return(model.Token{Model: gorm.Model{ID: 5}, UserID: 2}, nil).Times(1)
	tokenMock.EXPECT().DeleteToken("5").Return(nil).Times(1)

	api := &API{dao: dao.DaoWrapper{TokenDao: tokenMock}, log: slog.Default()}

	if _, err := api.DeleteToken(ctxAs(admin), &protobuf.DeleteTokenRequest{Id: 5}); err != nil {
		t.Fatalf("DeleteToken: %v", err)
	}
}

// As above: every caller already holds users.manage, so this cannot refuse in
// production, but the ownership check from v1 is kept rather than assumed dead.
func TestDeleteTokenRefusesANonOwnerWithoutPermission(t *testing.T) {
	lecturer := user(2, "Peter Prof", "peter@example.org", model.LecturerType)

	ctrl := gomock.NewController(t)
	tokenMock := mock_dao.NewMockTokenDao(ctrl)
	tokenMock.EXPECT().GetTokenByID("5").Return(model.Token{Model: gorm.Model{ID: 5}, UserID: 99}, nil).Times(1)
	tokenMock.EXPECT().DeleteToken(gomock.Any()).Times(0)

	api := &API{dao: dao.DaoWrapper{TokenDao: tokenMock}, log: slog.Default()}

	_, err := api.DeleteToken(ctxAs(lecturer), &protobuf.DeleteTokenRequest{Id: 5})

	if got := status.Code(err); got != codes.PermissionDenied {
		t.Errorf("code = %v, want %v", got, codes.PermissionDenied)
	}
}

func TestDeleteTokenReportsAMissingToken(t *testing.T) {
	admin := user(1, "Anja Admin", "anja@example.org", model.AdminType)

	ctrl := gomock.NewController(t)
	tokenMock := mock_dao.NewMockTokenDao(ctrl)
	tokenMock.EXPECT().GetTokenByID("5").Return(model.Token{}, gorm.ErrRecordNotFound).Times(1)

	api := &API{dao: dao.DaoWrapper{TokenDao: tokenMock}, log: slog.Default()}

	_, err := api.DeleteToken(ctxAs(admin), &protobuf.DeleteTokenRequest{Id: 5})

	if got := status.Code(err); got != codes.NotFound {
		t.Errorf("code = %v, want %v", got, codes.NotFound)
	}
}
