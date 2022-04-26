package dao

import (
	"github.com/joschahenningsen/TUM-Live/model"
	"gorm.io/gorm"
)

type CameraPresetDao interface {
	GetDefaultCameraPreset(lectureHallID uint) (res model.CameraPreset, err error)
}

type cameraPresetDao struct {
	db *gorm.DB
}

func NewCameraPresetDao() CameraPresetDao {
	return cameraPresetDao{db: DB}
}

func (d cameraPresetDao) GetDefaultCameraPreset(lectureHallID uint) (res model.CameraPreset, err error) {
	err = DB.Debug().First(&res, "lecture_hall_id = ? AND is_default", lectureHallID).Error
	return
}
