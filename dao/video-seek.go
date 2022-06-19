package dao

import (
	"github.com/joschahenningsen/TUM-Live/model"
	"gorm.io/gorm"
)

type VideoSeekDao interface {
	Add(videoSeekPoint model.VideoSeekPoint) error
}

type videoSeekDao struct {
	db *gorm.DB
}

func NewVideoSeekDao() VideoSeekDao {
	return videoSeekDao{db: DB}
}

func (d videoSeekDao) Add(videoSeekPoint model.VideoSeekPoint) error {
	return DB.Create(&videoSeekPoint).Error
}
