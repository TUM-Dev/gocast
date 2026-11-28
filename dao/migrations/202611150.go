package migrations

import (
	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

// Migrate202611150 hides the VoD stats recorded so far. AddVodView looked up the
// hour's row without filtering by stream, so each hour every VoD view landed on
// whichever stream was watched first. Per-course and per-lecture VoD numbers are
// meaningless and cannot be reattributed. The rows are soft-deleted, not dropped,
// so the all-course totals can still be recovered from the database.
func Migrate202611150() *gormigrate.Migration {
	return &gormigrate.Migration{
		ID: "202611150",
		Migrate: func(tx *gorm.DB) error {
			return tx.Exec("UPDATE stats SET deleted_at = CURRENT_TIMESTAMP WHERE live = ? AND deleted_at IS NULL", false).Error
		},
		Rollback: func(tx *gorm.DB) error {
			// Nothing else soft-deletes stats, so every deleted VoD row is one of ours.
			return tx.Exec("UPDATE stats SET deleted_at = NULL WHERE live = ? AND deleted_at IS NOT NULL", false).Error
		},
	}
}
