package dao

import (
	"errors"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/TUM-Dev/gocast/model"
)

//go:generate go tool mockgen -source=video-seek.go -destination ../mock_dao/video-seek.go

const maxChunksPerVideo = 150

// ErrPositionOutOfRange is returned when a reported seek position exceeds the stream duration.
var ErrPositionOutOfRange = errors.New("position is bigger than stream duration")

type VideoSeekDao interface {
	Add(streamID string, pos float64) error
	Get(streamID string) ([]model.VideoSeekChunk, error)
}

type videoSeekDao struct {
	db *gorm.DB
}

func NewVideoSeekDao() VideoSeekDao {
	return videoSeekDao{db: DB}
}

func (d videoSeekDao) Add(streamID string, pos float64) error {
	var stream *model.Stream
	if err := DB.First(&stream, "id = ?", streamID).Error; err != nil {
		return err
	}

	if (pos / float64(stream.Duration.Int32)) > 1 {
		return ErrPositionOutOfRange
	}

	chunkTimeRange := float64(stream.Duration.Int32) / maxChunksPerVideo
	chunk := uint(pos / chunkTimeRange)

	return DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "chunk_index"}, {Name: "stream_id"}},
		DoUpdates: clause.Assignments(map[string]interface{}{"hits": gorm.Expr("hits + 1")}),
	}).Create(&model.VideoSeekChunk{
		ChunkIndex: chunk,
		Hits:       1,
		StreamID:   stream.ID,
	}).Error
}

func (d videoSeekDao) Get(streamID string) ([]model.VideoSeekChunk, error) {
	var chunks []model.VideoSeekChunk

	if err := DB.Find(&chunks, "stream_id = ?", streamID).Error; err != nil {
		return nil, err
	}

	return chunks, nil
}
