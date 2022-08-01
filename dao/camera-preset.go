package dao

import (
	"context"
	"github.com/joschahenningsen/TUM-Live/model"
	"gorm.io/gorm"
)

//go:generate mockgen -source=camera-preset.go -destination ../mock_dao/camera-preset.go

type CameraPresetDao interface {
	GetDefaultCameraPreset(ctx context.Context, lectureHallID uint) (res model.CameraPreset, err error)
}

type cameraPresetDao struct {
	db *gorm.DB
}

func NewCameraPresetDao() CameraPresetDao {
	return cameraPresetDao{db: DB}
}

func (d cameraPresetDao) GetDefaultCameraPreset(ctx context.Context, lectureHallID uint) (res model.CameraPreset, err error) {
	err = DB.WithContext(ctx).Debug().First(&res, "lecture_hall_id = ? AND is_default", lectureHallID).Error
	return
}
