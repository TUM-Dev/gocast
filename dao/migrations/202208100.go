package migrations

import (
	"github.com/go-gormigrate/gormigrate/v2"
	"github.com/joschahenningsen/TUM-Live/model"
	"gorm.io/gorm"
)

// Migrate202210080 adjusts the user setting type to compensate for removal of 'enable_chromcast'.
func Migrate202210080() *gormigrate.Migration {
	return &gormigrate.Migration{
		ID: "202210083",
		Migrate: func(tx *gorm.DB) error {
			// Delete all chrome cast settings.
			err := tx.Unscoped().Model(&model.UserSetting{}).Where("type = 3").Delete(&model.UserSetting{}).Error
			if err != nil {
				return err
			}
			// CustomPlaybackSpeed previously was of type 4, since type 3 is deleted, it now becomes type 3.
			return tx.Model(&model.UserSetting{}).Where("type = 4").Update("type", 3).Error
		},
		Rollback: func(tx *gorm.DB) error {
			return nil
		},
	}
}
