package dao

import (
	"TUM-Live/model"
	"gorm.io/gorm/clause"
)

func SaveProgress(progress float64, userID uint, streamID uint) {
	// Update columns to new value on `id` conflict
	DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "stream_id"}, {Name: "user_id"}}, // key column
		DoUpdates: clause.AssignmentColumns([]string{"progress"}), // column needed to be updated
	}).Create(&model.StreamProgress{
		Progress: progress,
		StreamID: streamID,
		UserID:   userID,
	})
}

func LoadProgress(userID uint, streamID uint) float64 {
	var result float64
	DB.Raw("SELECT progress FROM stream_progresses WHERE user_id=? AND stream_id=?", userID, streamID).Scan(&result)
	return result
}
