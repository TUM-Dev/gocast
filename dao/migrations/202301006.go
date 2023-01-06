package migrations

import (
	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

// Migrate202301006 Drops the table "chat_user_likes".
func Migrate202301006() *gormigrate.Migration {
	return &gormigrate.Migration{
		ID: "202301006",
		Migrate: func(tx *gorm.DB) error {
			m := tx.Migrator()
			if m.HasTable("chat_user_likes") {
				return m.DropTable("chat_user_likes")
			}
			return nil
		},
		Rollback: func(tx *gorm.DB) error {
			return nil
		},
	}
}
