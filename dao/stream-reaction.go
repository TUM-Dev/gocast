package dao

import (
	"context"
	"time"

	"gorm.io/gorm"

	"github.com/TUM-Dev/gocast/model"
)

//go:generate mockgen -source=streamReaction.go -destination ../mock_dao/streamReaction.go

type StreamReactionDao interface {
	// Get StreamReaction by ID
	Get(context.Context, uint) (model.StreamReaction, error)

	// Create a new StreamReaction for the database
	Create(context.Context, *model.StreamReaction) error

	// Delete a StreamReaction by id.
	Delete(context.Context, uint) error

	GetByStream(context.Context, uint) ([]model.StreamReaction, error)

	GetByStreamWithinMinutes(context.Context, uint, uint) ([]model.StreamReaction, error)

	GetNumbersOfReactions(context.Context, uint) (map[string]int, error)

	GetLastReactionOfUser(context.Context, uint) (model.StreamReaction, error)
}

type streamReactionDao struct {
	db *gorm.DB
}

type reactionCount struct {
	Reaction string `json:"reaction"`
	Count    int    `json:"count"`
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

// Delete a StreamReaction by id.
func (d streamReactionDao) Delete(c context.Context, id uint) error {
	return d.db.WithContext(c).Delete(&model.StreamReaction{}, id).Error
}

// GetByStream gets a StreamReaction by stream.
func (d streamReactionDao) GetByStream(c context.Context, streamID uint) (res []model.StreamReaction, err error) {
	return res, d.db.WithContext(c).Where("stream_id = ?", streamID).Find(&res).Error
}

// GetByStream gets a StreamReaction by stream within the last ... minutes.
func (d streamReactionDao) GetByStreamWithinMinutes(c context.Context, streamID uint, minutes uint) (res []model.StreamReaction, err error) {
	time_specified := time.Now().Add(-time.Duration(minutes) * time.Minute)
	return res, d.db.WithContext(c).Where("stream_id = ? AND created_at > ?", streamID, time_specified).Find(&res).Error
}

// GetNumbersOfReactions gets the number of reactions grouped by reactions for a stream.
func (d streamReactionDao) GetNumbersOfReactions(c context.Context, streamID uint) (map[string]int, error) {
	var reactionCounts []reactionCount
	err := d.db.WithContext(c).Model(&model.StreamReaction{}).Where("stream_id = ?", streamID).Group("reaction").Select("reaction, count(reaction) as count").Scan(&reactionCounts).Error
	if err != nil {
		return nil, err
	}

	reactionMap := make(map[string]int)
	for _, rc := range reactionCounts {
		reactionMap[rc.Reaction] = rc.Count
	}

	return reactionMap, err
}

// GetLastReactionOfUser gets the last reaction of a user.
func (d streamReactionDao) GetLastReactionOfUser(c context.Context, userID uint) (res model.StreamReaction, err error) {
	return res, d.db.WithContext(c).Where("user_id = ?", userID).Last(&res).Error
}
