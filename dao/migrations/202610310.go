package migrations

import (
	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

// Migrate202610310 drops short_links. The feature redirected /<link> to a course's
// current or next lecture; nothing could create a link any more, and production held
// five, all from mid 2021. AutoMigrate never drops a table, so it goes here.
func Migrate202610310() *gormigrate.Migration {
	return &gormigrate.Migration{
		ID: "202610310",
		Migrate: func(tx *gorm.DB) error {
			return tx.Migrator().DropTable("short_links")
		},
		Rollback: func(tx *gorm.DB) error {
			// The links are not restored; the feature is gone.
			return nil
		},
	}
}
