package dao

import (
	"github.com/joschahenningsen/TUM-Live/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

//go:generate mockgen -source=progress.go -destination ../mock_dao/progress.go

var Progress = NewProgressDao()

type ProgressDao interface {
	SaveProgresses(progresses []model.StreamProgress) error
	LoadProgress(userID uint, streamID uint) (streamProgress model.StreamProgress, err error)
}

type progressDao struct {
	db *gorm.DB
}

func NewProgressDao() ProgressDao {
	return progressDao{db: DB}
}

// SaveProgresses saves a slice of stream progresses. If a progress already exists, it will be updated.
func (d progressDao) SaveProgresses(progresses []model.StreamProgress) error {
	return DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "stream_id"}, {Name: "user_id"}},   // key column
		DoUpdates: clause.AssignmentColumns([]string{"progress", "watched"}), // column needed to be updated
	}).Create(progresses).Error
}

// LoadProgress retrieves the current StreamProgress from the database for a given user and stream.
func (d progressDao) LoadProgress(userID uint, streamID uint) (streamProgress model.StreamProgress, err error) {
	err = DB.First(&streamProgress, "user_id = ? AND stream_id = ?", userID, streamID).Error
	return streamProgress, err
}
