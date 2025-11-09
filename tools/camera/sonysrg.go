package camera

import (
	"fmt"
	"net/http"

	uuid "github.com/satori/go.uuid"

	"github.com/TUM-Dev/gocast/model"
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
	_, _, err := makeAuthenticatedRequest(&s.Auth, "GET", "", fmt.Sprintf("%s/presetposition.cgi?PresetCall=%d", fmt.Sprintf(sonySRGBaseUrl, s.Ip), presetId))
	return err
}

func (s *SonySRG) TakeSnapshot(outDir string) (filename string, err error) {
	// oneshotimage1 takes JPEGs as still images from codec images corresponding to ImageCodec1 (Video Stream 1)
	logger.Info(fmt.Sprintf("%s/oneshotimage1", s.Ip))
	resp, _, err := makeAuthenticatedRequest(&s.Auth, "GET", "", fmt.Sprintf("http://%s/oneshotimage1", s.Ip))
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
	presets := make([]model.CameraPreset, 0, 16)
	for i := range 16 {
		_, status, err := makeAuthenticatedRequest(&s.Auth, "GET", "", fmt.Sprintf("http://%s/preset/presetimg%d.jpg", s.Ip, i+1))
		if err != nil || status != http.StatusOK {
			logger.Info("Preset not available, stop polling", "i", i, "err", err)
			break
		}
		presets = append(presets, model.CameraPreset{
			Name:      fmt.Sprintf("Preset %d", i+1),
			PresetID:  i + 1,
			IsDefault: i == 0,
		})
	}
	return presets, nil
}
