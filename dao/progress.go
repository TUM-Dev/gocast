package dao

import (
	"TUM-Live/model"

	"gorm.io/gorm/clause"
)

// SaveProgresses saves a slice of stream progresses. If a progress already exists, it will be updated.
func SaveProgresses(progresses []model.StreamProgress) error {
	return DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "stream_id"}, {Name: "user_id"}, {Name: "course_id"}}, // key column
		DoUpdates: clause.AssignmentColumns([]string{"progress", "watch_status"}),               // column needed to be updated
	}).Create(progresses).Error
}

// LoadProgress retrieves the current StreamProgress from the database for a given user and stream.
func LoadProgress(userID uint, streamID uint) (streamProgress model.StreamProgress, err error) {
	err = DB.First(&streamProgress, "user_id = ? AND stream_id = ?", userID, streamID).Error
	return streamProgress, err
}

// LoadProgressesForCourseAndUser retrieves all StreamProgresses for a given user and course.
func LoadProgressesForCourseAndUser(courseID uint, userID uint) (progresses []model.StreamProgress, err error) {
	err = DB.Where("course_id = ? AND user_id = ?", courseID, userID).Find(&progresses).Error
	return progresses, err
}
