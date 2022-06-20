package dao

import (
	"fmt"
	"github.com/joschahenningsen/TUM-Live/model"
	"gorm.io/gorm"
)

//go:generate mockgen -source=search.go -destination ../mock_dao/search.go

type SearchDao interface {
	Search(q string, courseId uint) ([]model.Stream, error)
}

type searchDao struct {
	db *gorm.DB
}

func NewSearchDao() SearchDao {
	return searchDao{db: DB}
}

func (d searchDao) Search(q string, courseId uint) ([]model.Stream, error) {
	var response []model.Stream
	partialQ := fmt.Sprintf("%s*", q)
	err := DB.
		Model(&model.Stream{}).
		Select("streams.*, "+
			"(match(streams.name) against(? in boolean mode)+"+
			"match(streams.description) against(? in boolean mode)+"+
			"match(c.message) against(? in boolean mode)+"+
			"match(vs.description) against(? in boolean mode)+"+
			"match(kw.text) against(? in boolean mode)"+
			") as 'relevance'", partialQ, partialQ, partialQ, partialQ).
		Joins("left join video_sections vs on vs.stream_id = streams.id").
		Joins("left join chats c on c.stream_id = streams.id").
		Joins("left join "+
			"(select e.text, e.stream_id from tumlive.keywords e where e.language = 'eng' "+
			"and e.stream_id in (select d.stream_id from tumlive.keywords d where d.language = 'deu')) kw "+
			"on kw.stream_id = streams.id").
		Where(
			"(match(streams.name) against(? in boolean mode) "+
				"OR match(streams.description) against(? in boolean mode) "+
				"OR match(c.message) against(? in boolean mode)"+
				"OR match(vs.description) against(? in boolean mode)"+
				"OR match(kw.text) against(? in boolean mode))"+
				"AND streams.course_id = ? AND streams.recording = 1",
			partialQ, partialQ, partialQ, partialQ, partialQ, partialQ, courseId).
		Group("streams.id").
		Order("relevance"). // Sort by 'relevance'
		Find(&response).
		Error
	return response, err
}
