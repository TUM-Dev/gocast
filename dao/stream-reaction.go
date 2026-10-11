package dao

import (
	"context"
	"time"

	"gorm.io/gorm"

	"github.com/TUM-Dev/gocast/model"
)

//go:generate mockgen -source=stream-reaction.go -destination ../mock_dao/stream-reaction.go

type StreamReactionDao interface {
	// Get StreamReaction by ID
	Get(context.Context, uint) (model.StreamReaction, error)

	// Create a new StreamReaction for the database
	Create(context.Context, *model.StreamReaction) error

	// Update an existing StreamReaction.
	Update(context.Context, *model.StreamReaction) error

	// Delete a StreamReaction by id.
	Delete(context.Context, uint) error

	GetByStreamWithinMinutes(context.Context, uint, uint) ([]model.StreamReaction, error)

	// GetByStreamAndUser gets the StreamReaction a user has for a stream, if any
	GetByStreamAndUser(context.Context, uint, uint) (model.StreamReaction, error)
}

type streamReactionDao struct {
	db *gorm.DB
}

func NewStreamReactionDao() StreamReactionDao {
	return streamReactionDao{db: DB}
}

// Get a StreamReaction by id.
func (d streamReactionDao) Get(c context.Context, id uint) (res model.StreamReaction, err error) {
	return res, d.db.WithContext(c).First(&res, id).Error
}

// Create a StreamReaction.
func (d streamReactionDao) Create(c context.Context, it *model.StreamReaction) error {
	return d.db.WithContext(c).Create(it).Error
}

// Update an existing StreamReaction. Also bumps UpdatedAt, which the recency
// window in GetByStreamWithinMinutes relies on.
func (d streamReactionDao) Update(c context.Context, it *model.StreamReaction) error {
	return d.db.WithContext(c).Save(it).Error
}

// Delete a StreamReaction by id.
func (d streamReactionDao) Delete(c context.Context, id uint) error {
	return d.db.WithContext(c).Delete(&model.StreamReaction{}, id).Error
}

// GetByStreamWithinMinutes gets a StreamReaction by stream that was last updated within the last ... minutes and filtering on updated_at
func (d streamReactionDao) GetByStreamWithinMinutes(c context.Context, streamID uint, minutes uint) (res []model.StreamReaction, err error) {
	time_specified := time.Now().Add(-time.Duration(minutes) * time.Minute)
	return res, d.db.WithContext(c).Where("stream_id = ? AND updated_at > ?", streamID, time_specified).Find(&res).Error
}

// GetByStreamAndUser gets the StreamReaction a user currently has for a stream, if any.
func (d streamReactionDao) GetByStreamAndUser(c context.Context, streamID uint, userID uint) (res model.StreamReaction, err error) {
	return res, d.db.WithContext(c).Where("stream_id = ? AND user_id = ?", streamID, userID).First(&res).Error
}
