// Package integrationaccount is an operator tool that provisions the
// Artemis integration service account and its long-lived bearer token.
//
// Operator runbook
// ================
// 1. Build:
//
//	go build -o provision-integration-account ./cmd/integrationaccount
//
// 2. Run (requires a reachable gocast database, same config as the main
// service):
//
//	./provision-integration-account \
//	    -login artemis-integration \
//	    -name  "Artemis Integration Service"
//
// 3. The command prints the bearer token ONCE to stdout:
//
//	service account user ID: 42
//	token: <hex-encoded 32-byte random token>
//
// 4. Hand the printed token to Artemis ops out-of-band (e.g. via a
// secrets manager or an encrypted channel). The token is stored as plain
// text in the database for lookup (gocast does not hash tokens); it is
// NEVER written to any log by this tool. Protect your database
// accordingly — anyone with DB read access can recover all service tokens.
//
// 5. Configure Artemis:
//
//	artemis.tum-live.service-account-token: <token>
//	artemis.tum-live.service-account-user-id: <user ID>
//
// The command is idempotent: if a ServiceType user with the same login
// already exists it is reused; every invocation always mints a fresh
// token.
package main

import (
	"database/sql"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"

	"crypto/rand"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/model"
	"github.com/TUM-Dev/gocast/tools"
)

const (
	tokenByteLen = 32 // 256 bits — sufficient entropy for a long-lived bearer token
)

func main() {
	login := flag.String("login", "artemis-integration", "LRZ-ID / login to use for the service account")
	name := flag.String("name", "Artemis Integration Service", "Display name for the service account")
	flag.Parse()

	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo})).
		With("service", "provision-integration-account")

	tools.LoadConfig()

	db, err := gorm.Open(mysql.Open(fmt.Sprintf(
		"%s:%s@tcp(%s:%d)/%s?parseTime=true&loc=Local",
		tools.Cfg.Db.User,
		tools.Cfg.Db.Password,
		tools.Cfg.Db.Host,
		tools.Cfg.Db.Port,
		tools.Cfg.Db.Database,
	)), &gorm.Config{})
	if err != nil {
		logger.Error("cannot open database", "err", err)
		os.Exit(1)
	}
	dao.DB = db

	tokenStr, userID, err := ProvisionServiceAccount(dao.NewUsersDao(), dao.NewTokenDao(), *login, *name)
	if err != nil {
		logger.Error("provision failed", "err", err)
		os.Exit(1)
	}

	// Print the token ONCE to stdout — never pass it through the logger.
	fmt.Printf("service account user ID: %d\n", userID)
	fmt.Printf("token: %s\n", tokenStr)
}

// ProvisionServiceAccount creates (or finds) a ServiceType user identified by
// login, atomically revokes all existing service-scoped tokens for that user,
// generates a fresh random bearer token, persists it, and returns the token
// string and the service user's ID.
//
// The revoke+insert is performed in a single database transaction via
// TokenDao.RotateServiceToken. If the insert fails the delete is rolled back,
// so a failed run can never leave the user with zero valid tokens.
//
// The token is returned by value so the caller can print it exactly once. It
// is NEVER passed to any log sink inside this function.
func ProvisionServiceAccount(usersDao dao.UsersDao, tokenDao dao.TokenDao, login, name string) (token string, userID uint, err error) {
	user, err := findOrCreateServiceUser(usersDao, login, name)
	if err != nil {
		return "", 0, fmt.Errorf("find/create service user: %w", err)
	}

	token, err = GenerateServiceToken()
	if err != nil {
		return "", 0, fmt.Errorf("generate token: %w", err)
	}

	// Atomically revoke all prior service-scoped tokens and insert the new one.
	// A failed insert rolls back the delete so no tokens are lost on error.
	if err = tokenDao.RotateServiceToken(user.ID, model.Token{
		UserID:  user.ID,
		Token:   token,
		Scope:   model.TokenScopeService,
		Expires: sql.NullTime{Valid: false}, // no expiration — long-lived service token
	}); err != nil {
		return "", 0, fmt.Errorf("rotate service token: %w", err)
	}

	return token, user.ID, nil
}

// findOrCreateServiceUser looks up a user with the given LRZ ID. If it does
// not exist it creates one with Role == ServiceType.
func findOrCreateServiceUser(usersDao dao.UsersDao, login, name string) (model.User, error) {
	existing, err := usersDao.GetUserByLrzID(login)
	if err == nil {
		// User already exists — validate it is a service account.
		if existing.Role != model.ServiceType {
			return model.User{}, fmt.Errorf("user %q exists but has role %d (expected ServiceType=%d)", login, existing.Role, model.ServiceType)
		}
		return existing, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return model.User{}, fmt.Errorf("lookup user by lrz_id: %w", err)
	}

	// User does not exist — create it.
	user := &model.User{
		Name:  name,
		LrzID: login,
		Role:  model.ServiceType,
	}
	if createErr := usersDao.CreateUser(nil, user); createErr != nil { //nolint:staticcheck // nil ctx is fine here; CreateUser does not use it
		return model.User{}, fmt.Errorf("create service user: %w", createErr)
	}
	return *user, nil
}

// GenerateServiceToken returns a cryptographically random, hex-encoded token
// suitable for use as a long-lived bearer token.  The raw entropy is
// tokenByteLen bytes (256 bits).  The resulting string is URL-safe and
// contains only hex characters [0-9a-f].
func GenerateServiceToken() (string, error) {
	b := make([]byte, tokenByteLen)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("crypto/rand.Read: %w", err)
	}
	return hex.EncodeToString(b), nil
}
