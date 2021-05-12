package tools

import (
	"TUM-Live/dao"
	"TUM-Live/model"
	"TUM-Live/tools/camera"
	"github.com/getsentry/sentry-go"
	"log"
)

//FetchCameraPresets Queries all cameras of lecture halls for their camera presets and saves them to the database
func FetchCameraPresets() {
	lectureHalls := dao.GetAllLectureHalls()
	for _, lectureHall := range lectureHalls {
		log.Printf("camera: %s", lectureHall.CameraIP)
		if lectureHall.CameraIP != "" {
			cam := camera.NewCamera(lectureHall.CameraIP, Cfg.CameraAuthentication)
			presets, err := cam.GetPresets()
			if err != nil {
				sentry.CaptureException(err)
				continue
			}
			for _, preset := range presets {
				preset.LectureHallId = lectureHall.ID
			}
			lectureHall.CameraPresets = presets
			dao.SaveLectureHallFullAssoc(lectureHall)
		}
	}
}

func UsePreset(preset model.CameraPreset) {
	lectureHall, err := dao.GetLectureHallByID(preset.LectureHallId)
	if err != nil {
		sentry.CaptureException(err)
		return
	}
	c := camera.NewCamera(lectureHall.CameraIP, Cfg.CameraAuthentication)
	err = c.SetPreset(preset.PresetID)
	if err != nil {
		log.Printf("%v", err)
	}
}
