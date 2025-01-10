package tools

import (
	"context"
	"errors"
	"time"

	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/model"
	"github.com/TUM-Dev/gocast/tools/camera"
	"github.com/getsentry/sentry-go"
)

//go:generate mockgen -source=presets.go -destination ../mock_tools/presets.go

type PresetUtility interface {
	FetchCameraPresets(context.Context)
	FetchLHPresets(model.LectureHall)
	UsePreset(model.CameraPreset)
	TakeSnapshot(model.CameraPreset)
	ProvideCamera(model.CameraType, string) (camera.Cam, error)
}

type presetUtility struct {
	LectureHallDao dao.LectureHallsDao
}

func NewPresetUtility(lectureHallDao dao.LectureHallsDao) PresetUtility {
	return presetUtility{lectureHallDao}
}

func (p presetUtility) ProvideCamera(ctype model.CameraType, ip string) (camera.Cam, error) {
	switch ctype {
	case model.Axis:
		return camera.NewAxisCam(ip, Cfg.Auths.CamAuth), nil
	case model.Panasonic:
		return camera.NewPanasonicCam(ip, nil), nil
	}
	return nil, errors.New("invalid camera type")
}

// FetchCameraPresets Queries all cameras of lecture halls for their camera presets and saves them to the database
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
		cam, err := p.ProvideCamera(lectureHall.CameraType, lectureHall.CameraIP)
		if err != nil {
			logger.Error("Error providing camera", "err", err)
			return
		}
		presets, err := cam.GetPresets()
		if err != nil {
			logger.Warn("FetchCameraPresets: failed to get Presets", "err", err, "AxisCam", lectureHall.CameraIP)
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
	lectureHall, err := p.LectureHallDao.GetLectureHallByID(preset.LectureHallID)
	if err != nil {
		sentry.CaptureException(err)
		return
	}
	cam, err := p.ProvideCamera(lectureHall.CameraType, lectureHall.CameraIP)
	if err != nil {
		logger.Error("Error providing camera", "err", err)
		return
	}
	err = cam.SetPreset(preset.PresetID)
	if err != nil {
		logger.Error("UsePreset: unable to set preset for camera", "err", err)
	}
}

// TakeSnapshot Creates an image for a preset. Saves it to the disk and database.
// Function is blocking and needs ~20 Seconds to complete! Only call in goroutine.
func (p presetUtility) TakeSnapshot(preset model.CameraPreset) {
	p.UsePreset(preset)
	time.Sleep(time.Second * 10)
	lectureHall, err := p.LectureHallDao.GetLectureHallByID(preset.LectureHallID)
	if err != nil {
		sentry.CaptureException(err)
		return
	}
	cam, err := p.ProvideCamera(lectureHall.CameraType, lectureHall.CameraIP)
	if err != nil {
		logger.Error("Error providing camera", "err", err)
		return
	}
	fileName, err := cam.TakeSnapshot(Cfg.Paths.Static)
	if err != nil {
		logger.Error("TakeSnapshot: failed to get camera snapshot", "err", err, "cam", lectureHall.CameraIP)
		return
	}
	preset.Image = fileName
	err = p.LectureHallDao.SavePreset(preset)
	if err != nil {
		logger.Error("TakeSnapshot: failed to save snapshot file", "err", err, "cam", lectureHall.CameraIP)
		return
	}
}
