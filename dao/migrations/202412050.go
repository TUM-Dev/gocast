package migrations

import (
	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"

	"github.com/TUM-Dev/gocast/model"
)

// Migrate202412050 creates the jobs table for tracking runner job status.
func Migrate202412050() *gormigrate.Migration {
	return &gormigrate.Migration{
		ID: "202412050",
		Migrate: func(tx *gorm.DB) error {
			return tx.AutoMigrate(&model.Job{})
		},
		Rollback: func(tx *gorm.DB) error {
			return tx.Migrator().DropTable("jobs")
		},
	}
}
