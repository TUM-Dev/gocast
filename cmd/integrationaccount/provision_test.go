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
// yet, ProvisionServiceAccount creates one, then calls RotateServiceToken with
// the correct UserID and Scope, and returns the token. The rotation is atomic —
// no separate DeleteServiceTokensForUser or AddToken calls are expected.
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

	// RotateServiceToken must be called with the correct userID and a token that
	// has the right Scope and is not expired. The delete+insert inside the real
	// implementation is wrapped in a DB transaction; the mock tests the contract.
	tokenDao.EXPECT().
		RotateServiceToken(fakeID, gomock.AssignableToTypeOf(model.Token{})).
		DoAndReturn(func(uid uint, tok model.Token) error {
			if tok.UserID != fakeID {
				t.Errorf("RotateServiceToken: expected UserID=%d, got %d", fakeID, tok.UserID)
			}
			if tok.Scope != model.TokenScopeService {
				t.Errorf("RotateServiceToken: expected Scope=%q, got %q", model.TokenScopeService, tok.Scope)
			}
			if tok.Token == "" {
				t.Error("RotateServiceToken: token string must not be empty")
			}
			if tok.Expires.Valid {
				t.Error("RotateServiceToken: Expires must be null (long-lived token)")
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
// user already exists, ProvisionServiceAccount reuses it and calls
// RotateServiceToken (atomic delete+insert). CreateUser must NOT be called.
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

	// RotateServiceToken is called exactly once: it atomically deletes old
	// service-scoped tokens and inserts the new one.
	tokenDao.EXPECT().
		RotateServiceToken(fakeID, gomock.AssignableToTypeOf(model.Token{})).
		DoAndReturn(func(uid uint, tok model.Token) error {
			if tok.UserID != fakeID {
				t.Errorf("RotateServiceToken: expected UserID=%d, got %d", fakeID, tok.UserID)
			}
			if tok.Scope != model.TokenScopeService {
				t.Errorf("RotateServiceToken: expected Scope=%q, got %q", model.TokenScopeService, tok.Scope)
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

// TestProvisionServiceAccount_RotationFails verifies that if RotateServiceToken
// returns an error, ProvisionServiceAccount propagates it. Because the rotation
// is atomic, a failed RotateServiceToken means the old tokens are NOT deleted
// (the real implementation rolls back the transaction); the mock confirms that
// ProvisionServiceAccount aborts immediately and returns the error.
func TestProvisionServiceAccount_RotationFails(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	usersDao := mock_dao.NewMockUsersDao(ctrl)
	tokenDao := mock_dao.NewMockTokenDao(ctrl)

	serviceUser := model.User{Role: model.ServiceType}
	serviceUser.ID = 5

	usersDao.EXPECT().
		GetUserByLrzID("artemis-integration").
		Return(serviceUser, nil)

	// RotateServiceToken fails (simulates insert error after delete; the real
	// implementation rolls back so old tokens remain — atomicity guarantee).
	tokenDao.EXPECT().
		RotateServiceToken(uint(5), gomock.AssignableToTypeOf(model.Token{})).
		Return(errors.New("db error during rotation"))

	_, _, err := ProvisionServiceAccount(usersDao, tokenDao, "artemis-integration", "Artemis")
	if err == nil {
		t.Fatal("expected error when token rotation fails, got nil")
	}
}

// TestProvisionServiceAccount_AtomicRotation_AddTokenError verifies the
// atomicity contract: when RotateServiceToken fails (simulating an AddToken
// error inside the transaction), old tokens must NOT have been deleted.
// We verify this by checking that RotateServiceToken is called exactly once
// and returns the error — the caller's DAO implementation guarantees rollback.
func TestProvisionServiceAccount_AtomicRotation_AddTokenError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	usersDao := mock_dao.NewMockUsersDao(ctrl)
	tokenDao := mock_dao.NewMockTokenDao(ctrl)

	serviceUser := model.User{Role: model.ServiceType}
	serviceUser.ID = 11

	usersDao.EXPECT().
		GetUserByLrzID("artemis-integration").
		Return(serviceUser, nil)

	addTokenErr := errors.New("insert failed — transaction rolled back")
	var rotateWasCalled bool

	// RotateServiceToken is the single atomic operation. When it returns an
	// error, the caller guarantees that the DB transaction was rolled back —
	// meaning old tokens are intact. No DeleteServiceTokensForUser or AddToken
	// calls should appear separately.
	tokenDao.EXPECT().
		RotateServiceToken(uint(11), gomock.AssignableToTypeOf(model.Token{})).
		DoAndReturn(func(uid uint, tok model.Token) error {
			rotateWasCalled = true
			return addTokenErr
		})

	_, _, err := ProvisionServiceAccount(usersDao, tokenDao, "artemis-integration", "Artemis")
	if err == nil {
		t.Fatal("expected error from RotateServiceToken failure, got nil")
	}
	if !rotateWasCalled {
		t.Fatal("RotateServiceToken was not called")
	}
	// Verify the error is propagated (not swallowed).
	if !errors.Is(err, addTokenErr) {
		t.Errorf("expected error to wrap %v, got %v", addTokenErr, err)
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
// from RotateServiceToken is propagated as an error.
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

	// RotateServiceToken encapsulates the atomic delete+insert; a DB error
	// anywhere inside it is propagated by ProvisionServiceAccount.
	tokenDao.EXPECT().
		RotateServiceToken(uint(99), gomock.Any()).
		Return(errors.New("db down"))

	_, _, err := ProvisionServiceAccount(usersDao, tokenDao, "artemis-integration", "Artemis")
	if err == nil {
		t.Fatal("expected error from RotateServiceToken failure, got nil")
	}
}
