package dao

import (
	"context"
	"fmt"
	"github.com/joschahenningsen/TUM-Live/model"
	"gorm.io/gorm"
)

//go:generate mockgen -source=search.go -destination ../mock_dao/search.go

type SearchDao interface {
	Search(ctx context.Context, q string, courseId uint) ([]model.Stream, error)
}

type searchDao struct {
	db *gorm.DB
}

func NewSearchDao() SearchDao {
	return searchDao{db: DB}
}

func (d searchDao) Search(ctx context.Context, q string, courseId uint) ([]model.Stream, error) {
	var response []model.Stream
	partialQ := fmt.Sprintf("%s*", q)
	subQuery := DB.WithContext(ctx).Raw(
		"? UNION ? UNION ? UNION ? UNION ?",
		d.db.WithContext(ctx).
			Select("DISTINCT streams.*, MATCH(keywords.text) AGAINST(?) 'relevance'", partialQ).
			Model(&model.Stream{}).
			Joins("JOIN keywords ON streams.id = keywords.stream_id").
			Where("MATCH(keywords.text) AGAINST(?)", partialQ),
		d.db.WithContext(ctx).
			Select("DISTINCT streams.*, MATCH(streams.name) AGAINST(?) 'relevance'", partialQ).
			Model(&model.Stream{}).
			Where("MATCH(streams.name) AGAINST(?)", partialQ),
		d.db.WithContext(ctx).
			Select("DISTINCT streams.*, MATCH(streams.description) AGAINST(?) 'relevance'", partialQ).
			Model(&model.Stream{}).
			Joins("JOIN keywords ON streams.id = keywords.stream_id").
			Where("MATCH(streams.description) AGAINST(?)", partialQ),
		d.db.WithContext(ctx).
			Select("DISTINCT streams.*, MATCH(chats.message) AGAINST(?) 'relevance'", partialQ).
			Model(&model.Stream{}).
			Joins("JOIN chats ON streams.id = chats.stream_id").
			Where("MATCH(chats.message) AGAINST(?)", partialQ),
		d.db.WithContext(ctx).
			Select("DISTINCT streams.*, MATCH(vs.description) AGAINST(?) 'relevance'", partialQ).
			Model(&model.Stream{}).
			Joins("JOIN video_sections vs ON streams.id = vs.stream_id").
			Where("MATCH(vs.description) AGAINST(?)", partialQ),
	)
	err := d.db.WithContext(ctx).
		Table("(?) as t", subQuery).
		Where("t.course_id = ? AND t.recording = 1 AND t.deleted_at IS NULL", courseId).
		Group("t.id").
		Order("t.relevance").
		Find(&response).Error
	return response, err
}
