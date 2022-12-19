package migrations

import (
	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

// Migrate202212020 Drops the table "prefetched_courses".
func Migrate202212020() *gormigrate.Migration {
	return &gormigrate.Migration{
		ID: "202212020",
		Migrate: func(tx *gorm.DB) error {
			m := tx.Migrator()
			if m.HasTable("prefetched_courses") {
				return m.DropTable("prefetched_courses")
			}
			return nil
		},
		Rollback: func(tx *gorm.DB) error {
			return nil
		},
	}
}
