package tools

import (
	"context"
	"github.com/getsentry/sentry-go"
	"github.com/joschahenningsen/TUM-Live/dao"
	"github.com/joschahenningsen/TUM-Live/model"
	"github.com/joschahenningsen/TUM-Live/tools/camera"
	log "github.com/sirupsen/logrus"
	"time"
)

//FetchCameraPresets Queries all cameras of lecture halls for their camera presets and saves them to the database
func FetchCameraPresets(ctx context.Context, lectureHallDao dao.LectureHallsDao) {
	span := sentry.StartSpan(ctx, "FetchCameraPresets")
	defer span.Finish()
	lectureHalls := lectureHallDao.GetAllLectureHalls()
	for _, lectureHall := range lectureHalls {
		FetchLHPresets(lectureHall, lectureHallDao)
	}
}

func FetchLHPresets(lectureHall model.LectureHall, lectureHallDao dao.LectureHallsDao) {
	if lectureHall.CameraIP != "" {
		cam := camera.NewCamera(lectureHall.CameraIP, Cfg.Auths.CamAuth)
		presets, err := cam.GetPresets()
		if err != nil {
			log.WithError(err).WithField("Camera", cam.Ip).Warn("FetchCameraPresets: failed to get Presets")
			return
		}
		/*for i := range presets {
			findExistingImageForPreset(&presets[i], lectureHall.CameraPresets)
			presets[i].LectureHallID = lectureHall.ID
		}*/
		lectureHall.CameraPresets = presets
		lectureHallDao.SaveLectureHallFullAssoc(lectureHall)
	}
}

func UsePreset(preset model.CameraPreset, lectureHallDao dao.LectureHallsDao) {
	lectureHall, err := lectureHallDao.GetLectureHallByID(preset.LectureHallID)
	if err != nil {
		sentry.CaptureException(err)
		return
	}
	c := camera.NewCamera(lectureHall.CameraIP, Cfg.Auths.CamAuth)
	err = c.SetPreset(preset.PresetID)
	if err != nil {
		log.WithError(err).Error("UsePreset: unable to set preset for camera")
	}
}

//TakeSnapshot Creates an image for a preset. Saves it to the disk and database.
//Function is blocking and needs ~20 Seconds to complete! Only call in goroutine.
func TakeSnapshot(preset model.CameraPreset, lectureHallDao dao.LectureHallsDao) {
	UsePreset(preset, lectureHallDao)
	time.Sleep(time.Second * 10)
	lectureHall, err := lectureHallDao.GetLectureHallByID(preset.LectureHallID)
	if err != nil {
		sentry.CaptureException(err)
		return
	}
	c := camera.NewCamera(lectureHall.CameraIP, Cfg.Auths.CamAuth)
	fileName, err := c.TakeSnapshot(Cfg.Paths.Static)
	if err != nil {
		log.WithField("Camera", c.Ip).WithError(err).Error("TakeSnapshot: failed to get camera snapshot")
		return
	}
	preset.Image = fileName
	err = lectureHallDao.SavePreset(preset)
	if err != nil {
		log.WithField("Camera", c.Ip).WithError(err).Error("TakeSnapshot: failed to save snapshot file")
		return
	}
}
