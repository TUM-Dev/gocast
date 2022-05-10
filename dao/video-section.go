package dao

import (
	"fmt"
	"github.com/joschahenningsen/TUM-Live/model"
	"gorm.io/gorm"
)

type VideoSectionDao interface {
	Create(sections []model.VideoSection) error
	Update(section *model.VideoSection) error
	Delete(videoSectionID uint) error
	GetByStreamId(streamID uint) ([]model.VideoSection, error)

	Search(q string) ([]uint, error)
}

type videoSectionDao struct {
	db *gorm.DB
}

func NewVideoSectionDao() VideoSectionDao {
	return videoSectionDao{db: DB}
}

func (d videoSectionDao) Create(sections []model.VideoSection) error {
	return DB.Create(&sections).Error
}

func (d videoSectionDao) Update(section *model.VideoSection) error {
	return DB.Updates(&section).Error
}

func (d videoSectionDao) Delete(videoSectionID uint) error {
	return DB.Delete(&model.VideoSection{}, "id = ?", videoSectionID).Error
}

func (d videoSectionDao) GetByStreamId(streamID uint) ([]model.VideoSection, error) {
	var sections []model.VideoSection
	err := DB.Order("start_hours, start_minutes, start_seconds ASC").Find(&sections, "stream_id = ?", streamID).Error
	return sections, err
}

func (d videoSectionDao) Search(q string) ([]uint, error) {
	var streamIds []uint
	partialQ := fmt.Sprintf("%s*", q)
	err := DB.
		Model(&model.VideoSection{}).
		Select("stream_id").
		Distinct("stream_id").
		Where("match(description) against(? in boolean mode)", partialQ).
		Find(&streamIds).
		Error
	return streamIds, err
}
