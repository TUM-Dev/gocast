package main

import (
	"context"
	"errors"
	"testing"

	"go.uber.org/mock/gomock"
	"gorm.io/gorm"

	mock_dao "github.com/TUM-Dev/gocast/mock_dao"
	"github.com/TUM-Dev/gocast/model"
)

// ---------------------------------------------------------------------------
// Token-generator unit tests
// ---------------------------------------------------------------------------

func TestGenerateServiceToken_NonEmpty(t *testing.T) {
	tok, err := GenerateServiceToken()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tok == "" {
		t.Fatal("token must not be empty")
	}
}

func TestGenerateServiceToken_Length(t *testing.T) {
	tok, err := GenerateServiceToken()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// tokenByteLen=32 bytes → 64 hex characters
	if len(tok) != tokenByteLen*2 {
		t.Fatalf("expected %d hex chars, got %d", tokenByteLen*2, len(tok))
	}
}

func TestGenerateServiceToken_URLSafe(t *testing.T) {
	tok, err := GenerateServiceToken()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for i, ch := range tok {
		if !((ch >= '0' && ch <= '9') || (ch >= 'a' && ch <= 'f')) {
			t.Fatalf("token[%d]=%q is not a hex character — token not URL-safe: %s", i, ch, tok)
		}
	}
}

func TestGenerateServiceToken_Distinct(t *testing.T) {
	const n = 100
	seen := make(map[string]struct{}, n)
	for i := range n {
		tok, err := GenerateServiceToken()
		if err != nil {
			t.Fatalf("iteration %d: unexpected error: %v", i, err)
		}
		if _, dup := seen[tok]; dup {
			t.Fatalf("duplicate token generated at iteration %d: %s", i, tok)
		}
		seen[tok] = struct{}{}
	}
}

// ---------------------------------------------------------------------------
// Provisioning logic tests (using mock_dao)
// ---------------------------------------------------------------------------

// TestProvisionServiceAccount_NewUser verifies that when no service user exists
// yet, ProvisionServiceAccount creates one, revokes any prior service-scoped
// tokens (none in this case), calls AddToken with Scope==TokenScopeService and
// the correct UserID, and returns the token.
func TestProvisionServiceAccount_NewUser(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	usersDao := mock_dao.NewMockUsersDao(ctrl)
	tokenDao := mock_dao.NewMockTokenDao(ctrl)

	const (
		login  = "artemis-integration"
		name   = "Artemis Integration Service"
		fakeID = uint(42)
	)

	// GetUserByLrzID returns not-found → the tool must create a new user.
	usersDao.EXPECT().
		GetUserByLrzID(login).
		Return(model.User{}, gorm.ErrRecordNotFound)

	// CreateUser is called; it sets the ID via pointer (simulate by returning nil and having the struct populated).
	usersDao.EXPECT().
		CreateUser(gomock.Any(), gomock.AssignableToTypeOf(&model.User{})).
		DoAndReturn(func(_ context.Context, u *model.User) error {
			// Simulate gorm setting the primary key after insert.
			u.ID = fakeID
			return nil
		})

	// DeleteServiceTokensForUser must be called before AddToken to revoke prior
	// service-scoped tokens (atomic rotation). For a brand-new user there are no
	// prior tokens, but the call must still happen.
	tokenDao.EXPECT().
		DeleteServiceTokensForUser(fakeID).
		Return(nil)

	// AddToken must be called with the correct scope and user ID.
	tokenDao.EXPECT().
		AddToken(gomock.AssignableToTypeOf(model.Token{})).
		DoAndReturn(func(tok model.Token) error {
			if tok.UserID != fakeID {
				t.Errorf("AddToken: expected UserID=%d, got %d", fakeID, tok.UserID)
			}
			if tok.Scope != model.TokenScopeService {
				t.Errorf("AddToken: expected Scope=%q, got %q", model.TokenScopeService, tok.Scope)
			}
			if tok.Token == "" {
				t.Error("AddToken: token string must not be empty")
			}
			if tok.Expires.Valid {
				t.Error("AddToken: Expires must be null (long-lived token)")
			}
			return nil
		})

	tokenStr, userID, err := ProvisionServiceAccount(usersDao, tokenDao, login, name)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if userID != fakeID {
		t.Errorf("returned userID=%d, want %d", userID, fakeID)
	}
	if tokenStr == "" {
		t.Error("returned token must not be empty")
	}
}

// TestProvisionServiceAccount_ExistingUser verifies that when a ServiceType
// user already exists, ProvisionServiceAccount reuses it, revokes old service
// tokens, and then mints a fresh one (atomic rotation). CreateUser must NOT be
// called.
func TestProvisionServiceAccount_ExistingUser(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	usersDao := mock_dao.NewMockUsersDao(ctrl)
	tokenDao := mock_dao.NewMockTokenDao(ctrl)

	const fakeID = uint(7)
	existingUser := model.User{Role: model.ServiceType}
	existingUser.ID = fakeID

	usersDao.EXPECT().
		GetUserByLrzID("artemis-integration").
		Return(existingUser, nil)

	// CreateUser must NOT be called.
	// (no EXPECT registered for CreateUser)

	// DeleteServiceTokensForUser must be called BEFORE AddToken so that the
	// rotation is atomic: old credentials are invalidated before a new one is
	// activated.
	tokenDao.EXPECT().
		DeleteServiceTokensForUser(fakeID).
		Return(nil)

	tokenDao.EXPECT().
		AddToken(gomock.AssignableToTypeOf(model.Token{})).
		DoAndReturn(func(tok model.Token) error {
			if tok.UserID != fakeID {
				t.Errorf("AddToken: expected UserID=%d, got %d", fakeID, tok.UserID)
			}
			if tok.Scope != model.TokenScopeService {
				t.Errorf("AddToken: expected Scope=%q, got %q", model.TokenScopeService, tok.Scope)
			}
			return nil
		})

	_, userID, err := ProvisionServiceAccount(usersDao, tokenDao, "artemis-integration", "Artemis Integration Service")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if userID != fakeID {
		t.Errorf("returned userID=%d, want %d", userID, fakeID)
	}
}

// TestProvisionServiceAccount_RevocationFails verifies that if
// DeleteServiceTokensForUser returns an error, ProvisionServiceAccount aborts
// and does not mint a new token. This ensures we never silently leave old
// service tokens active when revocation is broken.
func TestProvisionServiceAccount_RevocationFails(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	usersDao := mock_dao.NewMockUsersDao(ctrl)
	tokenDao := mock_dao.NewMockTokenDao(ctrl)

	serviceUser := model.User{Role: model.ServiceType}
	serviceUser.ID = 5

	usersDao.EXPECT().
		GetUserByLrzID("artemis-integration").
		Return(serviceUser, nil)

	tokenDao.EXPECT().
		DeleteServiceTokensForUser(uint(5)).
		Return(errors.New("db error during revocation"))

	// AddToken must NOT be called when revocation fails.

	_, _, err := ProvisionServiceAccount(usersDao, tokenDao, "artemis-integration", "Artemis")
	if err == nil {
		t.Fatal("expected error when token revocation fails, got nil")
	}
}

// TestProvisionServiceAccount_WrongRoleRejected verifies that if a user with
// the given login exists but has a non-ServiceType role, provisioning is
// rejected rather than silently reusing it.
func TestProvisionServiceAccount_WrongRoleRejected(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	usersDao := mock_dao.NewMockUsersDao(ctrl)
	tokenDao := mock_dao.NewMockTokenDao(ctrl)

	badUser := model.User{Role: model.AdminType}
	badUser.ID = 1

	usersDao.EXPECT().
		GetUserByLrzID("artemis-integration").
		Return(badUser, nil)

	// Neither CreateUser nor AddToken should be called.
	_, _, err := ProvisionServiceAccount(usersDao, tokenDao, "artemis-integration", "Artemis")
	if err == nil {
		t.Fatal("expected error when existing user has wrong role, got nil")
	}
}

// TestProvisionServiceAccount_TokenDaoError verifies that a database error
// from AddToken is propagated as an error.
func TestProvisionServiceAccount_TokenDaoError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	usersDao := mock_dao.NewMockUsersDao(ctrl)
	tokenDao := mock_dao.NewMockTokenDao(ctrl)

	serviceUser := model.User{Role: model.ServiceType}
	serviceUser.ID = 99

	usersDao.EXPECT().
		GetUserByLrzID("artemis-integration").
		Return(serviceUser, nil)

	// DeleteServiceTokensForUser is called first; it succeeds here.
	tokenDao.EXPECT().
		DeleteServiceTokensForUser(uint(99)).
		Return(nil)

	tokenDao.EXPECT().
		AddToken(gomock.Any()).
		Return(errors.New("db down"))

	_, _, err := ProvisionServiceAccount(usersDao, tokenDao, "artemis-integration", "Artemis")
	if err == nil {
		t.Fatal("expected error from AddToken failure, got nil")
	}
}
