package dao

import (
	"TUM-Live/model"
	"gorm.io/gorm/clause"
)

// SaveProgress saves the users progress for a given streamID in the database.
func SaveProgress(progress float64, userID uint, streamID uint) (err error) {
	// Update all columns, except primary keys, to new progress on conflict
	err = DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "stream_id"}, {Name: "user_id"}}, // key column
		DoUpdates: clause.AssignmentColumns([]string{"progress"}),          // column needed to be updated
	}).Create(&model.StreamProgress{
		Progress: progress,
		StreamID: streamID,
		UserID:   userID,
	}).Error
	return err
}

// LoadProgress retrieves the current StreamProgress from the database for a given user and stream.
func LoadProgress(userID uint, streamID uint) (streamProgress model.StreamProgress, err error) {
	err = DB.First(&streamProgress, "user_id = ? AND stream_id = ?", userID, streamID).Error
	return streamProgress, err
}
