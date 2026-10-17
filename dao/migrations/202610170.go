package migrations

import (
	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

// Migrate202610170 drops upload_keys. The table held the one-time keys a worker traded in
// for the stream of an uploaded recording. Runners hand out their own tokens, so nothing
// reads or writes it any more, and AutoMigrate never drops tables on its own.
func Migrate202610170() *gormigrate.Migration {
	return &gormigrate.Migration{
		ID: "202610170",
		Migrate: func(tx *gorm.DB) error {
			if !tx.Migrator().HasTable("upload_keys") {
				return nil
			}
			return tx.Migrator().DropTable("upload_keys")
		},
		Rollback: func(tx *gorm.DB) error {
			// The keys were single use and short lived: there is nothing to restore.
			return nil
		},
	}
}
