package dao

import (
	"gorm.io/gorm"

	"github.com/TUM-Dev/gocast/model"
)

//go:generate go tool mockgen -source=camera-preset.go -destination ../mock_dao/camera-preset.go

type CameraPresetDao interface {
	GetDefaultCameraPreset(lectureHallID uint) (res model.CameraPreset, err error)
}

type cameraPresetDao struct {
	db *gorm.DB
}

func NewCameraPresetDao() CameraPresetDao {
	return cameraPresetDao{db: DB}
}

func (d cameraPresetDao) GetDefaultCameraPreset(lectureHallID uint) (model.CameraPreset, error) {
	var res model.CameraPreset
	return res, DB.First(&res, "lecture_hall_id = ? AND is_default", lectureHallID).Error
}
