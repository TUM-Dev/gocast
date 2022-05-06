package dao

import "github.com/joschahenningsen/TUM-Live/model"

func GetDefaultCameraPreset(lectureHallID uint) (res model.CameraPreset, err error) {
	err = DB.Debug().First(&res, "lecture_hall_id = ? AND is_default", lectureHallID).Error
	return
}
