package tools

import (
	"TUM-Live/dao"
	"TUM-Live/model"
	"TUM-Live/tools/camera"
	"context"
	"github.com/getsentry/sentry-go"
	log "github.com/sirupsen/logrus"
	"time"
)

//FetchCameraPresets Queries all cameras of lecture halls for their camera presets and saves them to the database
func FetchCameraPresets(ctx context.Context) {
	span := sentry.StartSpan(ctx, "FetchCameraPresets")
	defer span.Finish()
	lectureHalls := dao.GetAllLectureHalls()
	for _, lectureHall := range lectureHalls {
		FetchLHPresets(lectureHall)
	}
}

func FetchLHPresets(lectureHall model.LectureHall) {
	if lectureHall.CameraIP != "" {
		cam := camera.NewCamera(lectureHall.CameraIP, Cfg.CameraAuthentication)
		presets, err := cam.GetPresets()
		if err != nil {
			log.WithError(err).WithField("Camera", cam.Ip).Warn("FetchCameraPresets: failed to get Presets")
			return
		}
		/*for i := range presets {
			findExistingImageForPreset(&presets[i], lectureHall.CameraPresets)
			presets[i].LectureHallId = lectureHall.ID
		}*/
		lectureHall.CameraPresets = presets
		dao.SaveLectureHallFullAssoc(lectureHall)
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
		log.WithError(err).Error("UsePreset: unable to set preset for camera")
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
		log.WithField("Camera", c.Ip).WithError(err).Error("TakeSnapshot: failed to get camera snapshot")
		return
	}
	preset.Image = fileName
	err = dao.SavePreset(preset)
	if err != nil {
		log.WithField("Camera", c.Ip).WithError(err).Error("TakeSnapshot: failed to save snapshot file")
		return
	}
}
