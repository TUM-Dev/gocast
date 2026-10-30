package migrations

import (
	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

// Migrate202610300 deletes the admin-scoped API tokens. They authenticated scripts
// against GET /api/stream/live, which is gone together with the scope, so a row left
// behind would sit in the token list as a credential that opens nothing. Lecturer
// tokens, which rtmp-proxy still takes, are untouched.
func Migrate202610300() *gormigrate.Migration {
	return &gormigrate.Migration{
		ID: "202610300",
		Migrate: func(tx *gorm.DB) error {
			if !tx.Migrator().HasTable("tokens") {
				return nil
			}
			// Hard delete: the secret is the row, and a soft-deleted one would keep it.
			return tx.Exec("DELETE FROM tokens WHERE scope = ?", "admin").Error
		},
		Rollback: func(tx *gorm.DB) error {
			// The scope grants nothing any more; there is nothing to restore.
			return nil
		},
	}
}
