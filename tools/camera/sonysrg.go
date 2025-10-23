package camera

import (
	"fmt"

	"github.com/TUM-Dev/gocast/model"
	uuid "github.com/satori/go.uuid"
)

/**
 *
 * Compatible cameras:
 * - Sony SRG-A40 series (untested)
 *
 **/

const sonySRGBaseUrl = "http://%s/command"

type SonySRG struct {
	Ip   string
	Auth string
}

func NewSonySRG(ip string, auth string) Cam {
	return &SonySRG{Ip: ip, Auth: auth}
}

func (s *SonySRG) SetPreset(presetId int) error {
	_, err := makeAuthenticatedRequest(&s.Auth, "GET", "", fmt.Sprintf("%s/presetposition.cgi?PresetCall=%d", fmt.Sprintf(sonySRGBaseUrl, s.Ip), presetId))
	return err
}

func (s *SonySRG) TakeSnapshot(outDir string) (filename string, err error) {
	// oneshotimage1 takes JPEGs as still images from codec images corresponding to ImageCodec1 (Video Stream 1)
	logger.Info(fmt.Sprintf("%s/oneshotimage1", s.Ip))
	resp, err := makeAuthenticatedRequest(&s.Auth, "GET", "", fmt.Sprintf("http://%s/oneshotimage1", s.Ip))
	if err != nil {
		return "", err
	}
	filename = uuid.NewV4().String() + ".jpg"
	err = saveResponseBuffer(outDir, filename, resp)
	if err != nil {
		return "", err
	}
	return filename, nil
}

func (s *SonySRG) GetPresets() ([]model.CameraPreset, error) {
	// Sony SRG-A40 cameras support up to 256 presets, but only a few are used (see panasonic.go)
	presets := make([]model.CameraPreset, 16)
	for i := range presets {
		presets[i].PresetID = i
		presets[i].Name = fmt.Sprintf("Preset %d", i)
		if i == 0 {
			presets[i].Name = "Home"
		}
	}
	return presets, nil
}
