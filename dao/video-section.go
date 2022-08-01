package dao

import (
	"context"
	"github.com/joschahenningsen/TUM-Live/model"
	"gorm.io/gorm"
)

//go:generate mockgen -source=video-section.go -destination ../mock_dao/video-section.go

type VideoSectionDao interface {
	Create(ctx context.Context, sections []model.VideoSection) error
	Update(ctx context.Context, section *model.VideoSection) error
	Delete(ctx context.Context, id uint) error
	Get(ctx context.Context, id uint) (model.VideoSection, error)
	GetByStreamId(ctx context.Context, id uint) ([]model.VideoSection, error)
}

type videoSectionDao struct {
	db *gorm.DB
}

func NewVideoSectionDao() VideoSectionDao {
	return videoSectionDao{db: DB}
}

func (d videoSectionDao) Create(ctx context.Context, sections []model.VideoSection) error {
	return d.db.WithContext(ctx).Create(&sections).Error
}

func (d videoSectionDao) Update(ctx context.Context, section *model.VideoSection) error {
	return d.db.WithContext(ctx).Session(&gorm.Session{FullSaveAssociations: true}).Updates(&section).Error
}

func (d videoSectionDao) Delete(ctx context.Context, videoSectionID uint) error {
	return d.db.WithContext(ctx).Delete(&model.VideoSection{}, "id = ?", videoSectionID).Error
}

func (d videoSectionDao) Get(ctx context.Context, videoSectionID uint) (section model.VideoSection, err error) {
	err = d.db.WithContext(ctx).Find(&section, "id = ?", videoSectionID).Error
	return section, err
}

func (d videoSectionDao) GetByStreamId(ctx context.Context, streamID uint) ([]model.VideoSection, error) {
	var sections []model.VideoSection
	err := DB.WithContext(ctx).Order("start_hours, start_minutes, start_seconds ASC").Find(&sections, "stream_id = ?", streamID).Error
	return sections, err
}
