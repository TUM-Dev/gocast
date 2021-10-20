package dao

import "TUM-Live/model"

func GetDefaultCameraPreset(lectureHallID uint) (res model.CameraPreset, err error) {
	err = DB.Model(&model.CameraPreset{}).Where("lecture_hall_id = ? AND default", lectureHallID).First(&res).Error
	return
}
