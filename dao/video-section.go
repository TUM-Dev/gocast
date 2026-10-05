package dao

import (
	"gorm.io/gorm"

	"github.com/TUM-Dev/gocast/model"
)

//go:generate go tool mockgen -source=video-section.go -destination ../mock_dao/video-section.go

type VideoSectionDao interface {
	Create([]model.VideoSection) error
	Update(*model.VideoSection) error
	// UpdateContent writes the section's description and start, zero values included,
	// which Update skips: it cannot move a section back to 0:00:00.
	UpdateContent(*model.VideoSection) error
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

func (d videoSectionDao) UpdateContent(section *model.VideoSection) error {
	return d.db.Model(section).
		Select("description", "start_hours", "start_minutes", "start_seconds").
		Updates(section).Error
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
