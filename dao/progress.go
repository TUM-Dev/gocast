package dao

import (
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/TUM-Dev/gocast/model"
)

//go:generate go tool mockgen -source=progress.go -destination ../mock_dao/progress.go

var Progress = NewProgressDao()

type ProgressDao interface {
	SaveProgress(progress model.StreamProgress, watched *bool) error
	GetProgressesForUser(userID uint) ([]model.StreamProgress, error)
	LoadProgress(userID uint, streamIDs []uint) (streamProgress []model.StreamProgress, err error)
}

type progressDao struct {
	db *gorm.DB
}

func NewProgressDao() ProgressDao {
	return progressDao{db: DB}
}

// GetProgressesForUser returns all stored progresses for a user.
func (d progressDao) GetProgressesForUser(userID uint) (r []model.StreamProgress, err error) {
	return r, DB.Where("user_id = ?", userID).Find(&r).Error
}

// SaveProgress stores a user's position in a stream, creating the row if needed.
//
// watched is a pointer because a plain progress report must leave the flag alone while
// an explicit false must clear it ("unmark as watched"); a bool cannot tell the two apart.
// The values are assigned explicitly rather than taken from the inserted row, because
// GORM leaves a zero value with a column default out of the INSERT.
func (d progressDao) SaveProgress(progress model.StreamProgress, watched *bool) error {
	updates := map[string]any{"progress": progress.Progress}
	if watched != nil {
		progress.Watched = *watched
		updates["watched"] = *watched
	} else {
		progress.Watched = false // only the value of a new row; an existing row keeps its flag
	}

	return DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "stream_id"}, {Name: "user_id"}}, // key column
		DoUpdates: clause.Assignments(updates),
	}).Create(&progress).Error
}

// LoadProgress retrieves the current StreamProgress from the database for a given user and stream.
func (d progressDao) LoadProgress(userID uint, streamIDs []uint) (streamProgress []model.StreamProgress, err error) {
	err = DB.Find(&streamProgress, "user_id = ? AND stream_id IN ?", userID, streamIDs).Error
	return streamProgress, err
}
