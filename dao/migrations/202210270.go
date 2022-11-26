package migrations

import (
	"github.com/go-gormigrate/gormigrate/v2"
	"github.com/joschahenningsen/TUM-Live/model"
	"gorm.io/gorm"
)

// Migrate202210270 Drops the column "paused" from streams.
func Migrate202210270() *gormigrate.Migration {
	return &gormigrate.Migration{
		ID: "202210270",
		Migrate: func(tx *gorm.DB) error {
			m := tx.Migrator()
			if m.HasColumn(&model.Stream{}, "paused") {
				return m.DropColumn(&model.Stream{}, "paused")
			}
			return nil
		},
		Rollback: func(tx *gorm.DB) error {
			return nil
		},
	}
}
