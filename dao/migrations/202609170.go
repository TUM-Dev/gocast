package migrations

import (
	"fmt"

	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

// Migrate202609170 backfills info_pages rows left with an empty slug from before the
// column existed (it was added NOT NULL with no default, so every pre-existing row got
// an empty string), which otherwise collide and block idx_info_pages_slug's unique index
// from being created. Falls back to the row's name, deduplicated with a numeric suffix on
// collision.
func Migrate202609170() *gormigrate.Migration {
	return &gormigrate.Migration{
		ID: "202609170",
		Migrate: func(tx *gorm.DB) error {
			m := tx.Migrator()
			if !m.HasTable("info_pages") || !m.HasColumn("info_pages", "slug") {
				return nil
			}

			type row struct {
				ID   uint
				Name string
			}
			var rows []row
			if err := tx.Table("info_pages").Where("slug = '' OR slug IS NULL").Find(&rows).Error; err != nil {
				return err
			}

			for _, r := range rows {
				slug := r.Name
				for suffix := 0; ; suffix++ {
					candidate := slug
					if suffix > 0 {
						candidate = fmt.Sprintf("%s-%d", slug, suffix)
					}
					var count int64
					if err := tx.Table("info_pages").Where("slug = ?", candidate).Count(&count).Error; err != nil {
						return err
					}
					if count == 0 {
						slug = candidate
						break
					}
				}
				if err := tx.Table("info_pages").Where("id = ?", r.ID).Update("slug", slug).Error; err != nil {
					return err
				}
			}
			return nil
		},
		Rollback: func(tx *gorm.DB) error {
			return nil
		},
	}
}
