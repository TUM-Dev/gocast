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

//go:generate mockgen -source=cameras.go -destination ../mock_tools/cameras.go

type PresetUtility interface {
	FetchCameraPresets(context.Context)
	FetchLHPresets(model.LectureHall)
	UsePreset(preset model.CameraPreset)
	TakeSnapshot(preset model.CameraPreset)
}

type presetUtility struct {
	LectureHallDao dao.LectureHallsDao
}

func NewPresetUtility(lectureHallDao dao.LectureHallsDao) PresetUtility {
	return presetUtility{lectureHallDao}
}

//FetchCameraPresets Queries all cameras of lecture halls for their camera presets and saves them to the database
func (p presetUtility) FetchCameraPresets(ctx context.Context) {
	span := sentry.StartSpan(ctx, "FetchCameraPresets")
	defer span.Finish()
	lectureHalls := p.LectureHallDao.GetAllLectureHalls()
	for _, lectureHall := range lectureHalls {
		p.FetchLHPresets(lectureHall)
	}
}

func (p presetUtility) FetchLHPresets(lectureHall model.LectureHall) {
	if lectureHall.CameraIP != "" {
		var cam camera.Cam
		switch lectureHall.CameraType {
		case model.Axis:
			cam = camera.NewAxisCam(lectureHall.CameraIP, Cfg.Auths.CamAuth)
		case model.Panasonic:
			cam = camera.NewPanasonicCam(lectureHall.CameraIP, nil)
		}
		presets, err := cam.GetPresets()
		if err != nil {
			log.WithError(err).WithField("AxisCam", lectureHall.CameraIP).Warn("FetchCameraPresets: failed to get Presets")
			return
		}
		/*for i := range presets {
			findExistingImageForPreset(&presets[i], lectureHall.CameraPresets)
			presets[i].LectureHallId = lectureHall.ID
		}*/
		lectureHall.CameraPresets = presets
		p.LectureHallDao.SaveLectureHallFullAssoc(lectureHall)
	}
}

func (p presetUtility) UsePreset(preset model.CameraPreset) {
	lectureHall, err := p.LectureHallDao.GetLectureHallByID(preset.LectureHallId)
	if err != nil {
		sentry.CaptureException(err)
		return
	}
	var cam camera.Cam
	switch lectureHall.CameraType {
	case model.Axis:
		cam = camera.NewAxisCam(lectureHall.CameraIP, Cfg.Auths.CamAuth)
	case model.Panasonic:
		cam = camera.NewPanasonicCam(lectureHall.CameraIP, nil)
	}
	err = cam.SetPreset(preset.PresetID)
	if err != nil {
		log.WithError(err).Error("UsePreset: unable to set preset for camera")
	}
}

//TakeSnapshot Creates an image for a preset. Saves it to the disk and database.
//Function is blocking and needs ~20 Seconds to complete! Only call in goroutine.
func (p presetUtility) TakeSnapshot(preset model.CameraPreset) {
	p.UsePreset(preset)
	time.Sleep(time.Second * 10)
	lectureHall, err := p.LectureHallDao.GetLectureHallByID(preset.LectureHallId)
	if err != nil {
		sentry.CaptureException(err)
		return
	}
	var cam camera.Cam
	switch lectureHall.CameraType {
	case model.Axis:
		cam = camera.NewAxisCam(lectureHall.CameraIP, Cfg.Auths.CamAuth)
	case model.Panasonic:
		cam = camera.NewPanasonicCam(lectureHall.CameraIP, nil)
	}
	fileName, err := cam.TakeSnapshot(Cfg.Paths.Static)
	if err != nil {
		log.WithField("cam", lectureHall.CameraIP).WithError(err).Error("TakeSnapshot: failed to get camera snapshot")
		return
	}
	preset.Image = fileName
	err = p.LectureHallDao.SavePreset(preset)
	if err != nil {
		log.WithField("cam", lectureHall.CameraIP).WithError(err).Error("TakeSnapshot: failed to save snapshot file")
		return
	}
}
