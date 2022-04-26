package dao

import (
	"github.com/golang/mock/gomock"
	"github.com/joschahenningsen/TUM-Live/mock_dao"
	"github.com/joschahenningsen/TUM-Live/model"
	"testing"
)

func TestGetDefaultCameraPreset(t *testing.T) {
	ctrl := gomock.NewController(t)

	m := mock_dao.NewMockCameraPresetDao(ctrl)

	var lectureHallId uint = 1

	expectedPreset := model.CameraPreset{
		Name:          "Home",
		PresetID:      1,
		Image:         "41ed6288-0a96-410d-89f5-98ee53b0176f.jpg",
		LectureHallId: lectureHallId,
		IsDefault:     true}

	m.EXPECT().GetDefaultCameraPreset(lectureHallId).Return(expectedPreset, nil).Times(1)
}
