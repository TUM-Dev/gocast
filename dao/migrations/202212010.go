package migrations

import (
	"github.com/TUM-Dev/gocast/model"
	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

// Migrate202212010 changes data type of the 'name' column in the 'user' table from 'longtext' to 'varchar(80)'
func Migrate202212010() *gormigrate.Migration {
	return &gormigrate.Migration{
		ID: "202212010",
		Migrate: func(tx *gorm.DB) error {
			err := tx.Set(`gorm:"type:varchar(80); not null" json:"name"`, "ENGINE=InnoDB").AutoMigrate(&model.User{})
			if err != nil {
				return err
			}
			return nil
		},
		Rollback: func(tx *gorm.DB) error {
			return nil
		},
	}
}
