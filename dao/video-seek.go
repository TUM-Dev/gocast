package dao

import (
	"github.com/joschahenningsen/TUM-Live/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const maxChunksPerVideo = 150

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

	hitPos := float64(stream.Duration) * pos
	chunkTimeRange := float64(stream.Duration) / maxChunksPerVideo
	chunk := uint(hitPos / chunkTimeRange)

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
