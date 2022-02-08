package migrations

import (
	"TUM-Live/model"
	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

// Migrate202201280 Deletes all messages longer than 200 characters and all empty messages
func Migrate202201280() *gormigrate.Migration {
	return &gormigrate.Migration{
		ID: "202201280",
		Migrate: func(tx *gorm.DB) error {
			err := tx.Where("LENGTH(message) > 200").Delete(&model.Chat{}).Error // clean up messages from before length limit
			if err != nil {
				return err
			}
			return tx.Where("REPLACE(message, ' ', '') = ''").Delete(&model.Chat{}).Error // clean up messages from before empty constraint
		},
		Rollback: func(tx *gorm.DB) error {
			return nil
		},
	}
}
