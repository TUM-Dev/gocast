package camera

import (
	"fmt"

	"github.com/TUM-Dev/gocast/model"
	"github.com/TUM-Dev/gocast/pkg/camera/axis"
	"github.com/TUM-Dev/gocast/pkg/camera/panasonic"
	"github.com/TUM-Dev/gocast/pkg/camera/sony"
)

//go:generate mockgen -source=controler.go -destination=mock/controler.go

type Service struct {
	auths map[model.CameraType]string
}

type Cam interface {
	// SetPreset moves the camera to the preset identified by preset.
	SetPreset(presetId int) error
	// TakeSnapshot creates a snapshot and returns the filename of it.
	TakeSnapshot(outDir string) (filename string, err error)
	// GetPresets fetches all available presets
	GetPresets() ([]model.CameraPreset, error)
}

func NewService(auths map[model.CameraType]string) *Service {
	return &Service{
		auths: auths,
	}
}

func (s *Service) For(addr string, cameraType model.CameraType) (Cam, error) {
	auth, hasAuth := s.auths[cameraType]
	if !hasAuth {
		auth = ""
	}
	switch cameraType {
	case model.Axis:
		return axis.NewAxisCam(addr, auth), nil
	case model.Panasonic:
		return panasonic.NewPanasonicCam(addr, auth), nil
	case model.Sony_SRG_A40:
		return sony.NewSonySRG(addr, auth), nil
	default:
		return nil, fmt.Errorf("unknown camera type")
	}
}
