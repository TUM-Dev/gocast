package dao

import (
	"context"
	"errors"
	"github.com/joschahenningsen/TUM-Live/model"
	"gorm.io/gorm"
)

//go:generate mockgen -source=subtitles.go -destination ../mock_dao/subtitles.go

type SubtitlesDao interface {
	// Get Subtitles by ID
	Get(context.Context, uint) (model.Subtitles, error)

	// GetByStreamIDandLang returns the subtitles for a given query
	GetByStreamIDandLang(context.Context, uint, string) (model.Subtitles, error)

	// CreateOrUpsert creates or updates subtitles for the database
	CreateOrUpsert(context.Context, *model.Subtitles) error

	// Create saves subtitles
	Create(c context.Context, it *model.Subtitles) error

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
	return res, DB.WithContext(c).First(&res, id).Error
}

func (d subtitlesDao) GetByStreamIDandLang(c context.Context, id uint, lang string) (res model.Subtitles, err error) {
	return res, DB.WithContext(c).First(&res, &model.Subtitles{StreamID: id, Language: lang}).Error
}

// CreateOrUpsert creates or upserts subtitles.
func (d subtitlesDao) CreateOrUpsert(c context.Context, it *model.Subtitles) error {
	update := DB.
		WithContext(c).
		Model(&model.Subtitles{}).Where("stream_id = ? AND language = ?", it.StreamID, it.Language).
		Update("content", it.Content)

	if update.Error != nil {
		if errors.Is(update.Error, gorm.ErrRecordNotFound) {
			return d.Create(c, it)
		}

		return update.Error
	}

	if update.RowsAffected == 0 {
		return d.Create(c, it)
	}

	return nil
}

// Create subtitles.
func (d subtitlesDao) Create(c context.Context, it *model.Subtitles) error {
	return DB.WithContext(c).Create(it).Error
}

// Delete a Subtitles by id.
func (d subtitlesDao) Delete(c context.Context, id uint) error {
	return DB.WithContext(c).Delete(&model.Subtitles{}, id).Error
}
