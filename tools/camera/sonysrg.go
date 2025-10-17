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
	addr string
	auth string
}

func NewSonySRG(addr string, auth string) SonySRG {
	return SonySRG{addr: addr, auth: auth}
}

func (s SonySRG) SetPreset(presetId int) error {
	auth := &s.auth
	if s.auth == "" {
		auth = nil
	}
	_, err := makeAuthenticatedRequest(auth, "GET", "", fmt.Sprintf("%s/presetposition.cgi?PresetCall=%d", fmt.Sprintf(sonySRGBaseUrl, s.addr), presetId))
	return err
}

func (s SonySRG) TakeSnapshot(outDir string) (filename string, err error) {
	auth := &s.auth
	if s.auth == "" {
		auth = nil
	}

	// oneshotimage1 takes JPEGs as still images from codec images corresponding to ImageCodec1 (Video Stream 1)
	logger.Info(fmt.Sprintf("%s/oneshotimage1", s.addr))
	resp, err := makeAuthenticatedRequest(auth, "GET", "", fmt.Sprintf("http://%s/oneshotimage1", s.addr))
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

func (s SonySRG) GetPresets() ([]model.CameraPreset, error) {
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
