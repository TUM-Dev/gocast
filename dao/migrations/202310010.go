package migrations

import (
	"github.com/TUM-Dev/gocast/model"
	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

// Migrate202310010 Adds the content of "name" to "display_name" and Drops the column "name".
func Migrate202310010() *gormigrate.Migration {
	return &gormigrate.Migration{
		ID: "202310010",
		Migrate: func(tx *gorm.DB) error {
			m := tx.Migrator()
			if m.HasColumn(&model.User{}, "name") {
				tx.Exec("UPDATE users SET display_name = name WHERE 1")
			}
			return m.DropColumn(&model.User{}, "name")
		},
		Rollback: func(tx *gorm.DB) error {
			return nil
		},
	}
}
