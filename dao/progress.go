package dao

import (
	"github.com/TUM-Dev/gocast/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

//go:generate mockgen -source=progress.go -destination ../mock_dao/progress.go

var Progress = NewProgressDao()

type ProgressDao interface {
	SaveProgresses(progresses []model.StreamProgress) error
	GetProgressesForUser(userID uint) ([]model.StreamProgress, error)
	LoadProgress(userID uint, streamIDs []uint) (streamProgress []model.StreamProgress, err error)
	SaveWatchedState(progress *model.StreamProgress) error
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

func filterProgress(progresses []model.StreamProgress, watched bool) []model.StreamProgress {
	var result []model.StreamProgress
	for _, progress := range progresses {
		if progress.Watched == watched {
			result = append(result, progress)
		}
	}
	return result
}

// SaveProgresses saves a slice of stream progresses. If a progress already exists, it will be updated.
// We need two different methods for that because else the watched state will be overwritten.
func (d progressDao) SaveProgresses(progresses []model.StreamProgress) error {
	noWatched := filterProgress(progresses, false)
	watched := filterProgress(progresses, true)
	var err error
	if len(noWatched) > 0 {
		err = DB.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "stream_id"}, {Name: "user_id"}}, // key column
			DoUpdates: clause.AssignmentColumns([]string{"progress"}),          // column needed to be updated
		}).Create(noWatched).Error
	}
	var err2 error

	if len(watched) > 0 {
		err2 = DB.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "stream_id"}, {Name: "user_id"}},   // key column
			DoUpdates: clause.AssignmentColumns([]string{"progress", "watched"}), // column needed to be updated
		}).Create(watched).Error
	}

	if err != nil {
		return err
	}
	return err2
}

// SaveWatchedState creates/updates a stream progress with its corresponding watched state.
func (d progressDao) SaveWatchedState(progress *model.StreamProgress) error {
	return DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "stream_id"}, {Name: "user_id"}}, // key column
		DoUpdates: clause.AssignmentColumns([]string{"watched"}),           // column needed to be updated
	}).Create(progress).Error
}

// LoadProgress retrieves the current StreamProgress from the database for a given user and stream.
func (d progressDao) LoadProgress(userID uint, streamIDs []uint) (streamProgress []model.StreamProgress, err error) {
	err = DB.Find(&streamProgress, "user_id = ? AND stream_id IN ?", userID, streamIDs).Error
	return streamProgress, err
}
