package migrations

import (
	"github.com/go-gormigrate/gormigrate/v2"
	"github.com/TUM-Dev/gocast/model"
	"gorm.io/gorm"
)

// Migrate202207240 Drops the column "stream_status" from streams, because they are superseded by transcodingProgresses
func Migrate202207240() *gormigrate.Migration {
	return &gormigrate.Migration{
		ID: "202207240",
		Migrate: func(tx *gorm.DB) error {
			m := tx.Migrator()
			if m.HasColumn(&model.Stream{}, "stream_status") {
				return m.DropColumn(&model.Stream{}, "stream_status")
			}
			return nil
		},
		Rollback: func(tx *gorm.DB) error {
			return nil
		},
	}
}
