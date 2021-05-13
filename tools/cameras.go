package tools

import (
	"TUM-Live/dao"
	"TUM-Live/model"
	"TUM-Live/tools/camera"
	"github.com/getsentry/sentry-go"
	"log"
	"time"
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

//TakeSnapshot Creates an image for a preset. Saves it to the disk and database.
//Function is blocking and needs ~20 Seconds to complete! Only call in goroutine.
func TakeSnapshot(preset model.CameraPreset) {
	UsePreset(preset)
	time.Sleep(time.Second * 10)
	lectureHall, err := dao.GetLectureHallByID(preset.LectureHallId)
	if err != nil {
		sentry.CaptureException(err)
		return
	}
	c := camera.NewCamera(lectureHall.CameraIP, Cfg.CameraAuthentication)
	fileName, err := c.TakeSnapshot(Cfg.StaticPath)
	if err != nil {
		log.Printf("%v", err)
		sentry.CaptureException(err)
		return
	}
	preset.Image = fileName
	err = dao.SavePreset(preset)
	if err != nil {
		log.Printf("failed to save preset image: %v", err)
		sentry.CaptureException(err)
		return
	}
}
