package migrations

import (
	"github.com/go-gormigrate/gormigrate/v2"
	"github.com/TUM-Dev/gocast/model"
	"gorm.io/gorm"
)

// Migrate202208110 Drops the column "live_enabled" from courses, since it isn't needed anymore
func Migrate202208110() *gormigrate.Migration {
	return &gormigrate.Migration{
		ID: "202208110",
		Migrate: func(tx *gorm.DB) error {
			m := tx.Migrator()
			if m.HasColumn(&model.Course{}, "live_enabled") {
				return m.DropColumn(&model.Course{}, "live_enabled")
			}
			return nil
		},
		Rollback: func(tx *gorm.DB) error {
			return nil
		},
	}
}
