package dao

import (
	"context"
	"github.com/joschahenningsen/TUM-Live/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

//go:generate mockgen -source=progress.go -destination ../mock_dao/progress.go

var Progress = NewProgressDao()

type ProgressDao interface {
	SaveProgresses(ctx context.Context, progresses []model.StreamProgress) error
	GetProgressesForUser(ctx context.Context, userID uint) ([]model.StreamProgress, error)
	LoadProgress(ctx context.Context, userID uint, streamID uint) (streamProgress model.StreamProgress, err error)
	SaveWatchedState(ctx context.Context, progress *model.StreamProgress) error
}

type progressDao struct {
	db *gorm.DB
}

func NewProgressDao() ProgressDao {
	return progressDao{db: DB}
}

// GetProgressesForUser returns all stored progresses for a user.
func (d progressDao) GetProgressesForUser(ctx context.Context, userID uint) (r []model.StreamProgress, err error) {
	return r, DB.WithContext(ctx).Where("user_id = ?", userID).Find(&r).Error
}

// SaveProgresses saves a slice of stream progresses. If a progress already exists, it will be updated.
func (d progressDao) SaveProgresses(ctx context.Context, progresses []model.StreamProgress) error {
	return DB.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "stream_id"}, {Name: "user_id"}}, // key column
		DoUpdates: clause.AssignmentColumns([]string{"progress"}),          // column needed to be updated
	}).Create(progresses).Error
}

// SaveWatchedState creates/updates a stream progress with its corresponding watched state.
func (d progressDao) SaveWatchedState(ctx context.Context, progress *model.StreamProgress) error {
	return DB.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "stream_id"}, {Name: "user_id"}}, // key column
		DoUpdates: clause.AssignmentColumns([]string{"watched"}),           // column needed to be updated
	}).Create(progress).Error
}

// LoadProgress retrieves the current StreamProgress from the database for a given user and stream.
func (d progressDao) LoadProgress(ctx context.Context, userID uint, streamID uint) (streamProgress model.StreamProgress, err error) {
	err = DB.WithContext(ctx).First(&streamProgress, "user_id = ? AND stream_id = ?", userID, streamID).Error
	return streamProgress, err
}
