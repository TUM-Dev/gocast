package dao

import (
	"github.com/joschahenningsen/TUM-Live/model"
	"gorm.io/gorm"
)

//go:generate mockgen -source=video-section.go -destination ../mock_dao/video-section.go

type VideoSectionDao interface {
	Create([]model.VideoSection) error
	Update(*model.VideoSection) error
	Delete(uint) error
	Get(uint) (model.VideoSection, error)
	GetByStreamId(uint) ([]model.VideoSection, error)
}

type videoSectionDao struct {
	db *gorm.DB
}

func NewVideoSectionDao() VideoSectionDao {
	return videoSectionDao{db: DB}
}

func (d videoSectionDao) Create(sections []model.VideoSection) error {
	return d.db.Create(&sections).Error
}

func (d videoSectionDao) Update(section *model.VideoSection) error {
	return d.db.Session(&gorm.Session{FullSaveAssociations: true}).Updates(&section).Error
}

func (d videoSectionDao) Delete(videoSectionID uint) error {
	return d.db.Delete(&model.VideoSection{}, "id = ?", videoSectionID).Error
}

func (d videoSectionDao) Get(videoSectionID uint) (section model.VideoSection, err error) {
	err = d.db.Find(&section, "id = ?", videoSectionID).Error
	return section, err
}

func (d videoSectionDao) GetByStreamId(streamID uint) ([]model.VideoSection, error) {
	var sections []model.VideoSection
	err := DB.Order("start_hours, start_minutes, start_seconds ASC").Find(&sections, "stream_id = ?", streamID).Error
	return sections, err
}
