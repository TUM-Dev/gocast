package dao

import (
	"context"
	"github.com/joschahenningsen/TUM-Live/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

//go:generate mockgen -source=subtitles.go -destination ../mock_dao/subtitles.go

type SubtitlesDao interface {
	// Get Subtitles by ID
	Get(context.Context, uint) (model.Subtitles, error)

	// GetByStreamIDandLang returns the subtitles for a given query
	GetByStreamIDandLang(context.Context, uint, string) (model.Subtitles, error)

	// Create or Update a new Subtitles for the database
	CreateOrUpsert(context.Context, *model.Subtitles) error

	// Delete a Subtitles by id.
	Delete(context.Context, uint) error
}

type subtitlesDao struct {
	db *gorm.DB
}

func NewSubtitlesDao() SubtitlesDao {
	return subtitlesDao{db: DB}
}

// Get a Subtitles by id.
func (d subtitlesDao) Get(c context.Context, id uint) (res model.Subtitles, err error) {
	return res, d.db.WithContext(c).First(&res, id).Error
}

func (d subtitlesDao) GetByStreamIDandLang(c context.Context, id uint, lang string) (res model.Subtitles, err error) {
	return res, d.db.WithContext(c).First(&res, &model.Subtitles{StreamID: id, Language: lang}).Error
}

// CreateOrUpsert creates or upserts subtitles.
func (d subtitlesDao) CreateOrUpsert(c context.Context, it *model.Subtitles) error {
	return d.db.Clauses(clause.OnConflict{UpdateAll: true}).WithContext(c).Create(it).WithContext(c).Error
}

// Delete a Subtitles by id.
func (d subtitlesDao) Delete(c context.Context, id uint) error {
	return d.db.WithContext(c).Delete(&model.Subtitles{}, id).Error
}
