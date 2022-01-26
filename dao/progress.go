package dao

import (
	"TUM-Live/model"
	"gorm.io/gorm/clause"
)

// SaveProgresses saves a slice of stream progresses. If a progress already exists, it will be updated.
func SaveProgresses(progresses []model.StreamProgress) error {
	return DB.Debug().Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "stream_id"}, {Name: "user_id"}}, // key column
		DoUpdates: clause.AssignmentColumns([]string{"progress"}),          // column needed to be updated
	}).Create(progresses).Error
}

// LoadProgress retrieves the current StreamProgress from the database for a given user and stream.
func LoadProgress(userID uint, streamID uint) (streamProgress model.StreamProgress, err error) {
	err = DB.First(&streamProgress, "user_id = ? AND stream_id = ?", userID, streamID).Error
	return streamProgress, err
}
