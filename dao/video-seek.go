package dao

import (
	"github.com/joschahenningsen/TUM-Live/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"time"
)

// 90 Minutes lecture would have worst case of 1080 chunks
const chunkSize = time.Second * 5

type VideoSeekDao interface {
	Add(streamID string, pos float64) error
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

	chunk := uint((float64(stream.Duration) / 100 * pos) / float64(chunkSize))

	return DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "chunk_index"}, {Name: "stream_id"}},
		DoUpdates: clause.Assignments(map[string]interface{}{"hits": gorm.Expr("hits + 1")}),
	}).Create(&model.VideoSeekChunk{
		ChunkIndex: chunk,
		Hits:       1,
		StreamID:   stream.ID,
	}).Error
}
