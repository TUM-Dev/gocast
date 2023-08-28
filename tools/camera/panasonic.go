package camera

import (
	"fmt"
	"github.com/TUM-Dev/gocast/model"
	uuid "github.com/satori/go.uuid"
	log "github.com/sirupsen/logrus"
)

/**
*
* Compatible cameras:
* - Panasonic HE40 series
* - Panasonic UE70 series (untested)
* - Panasonic HE42 series (untested)
*
**/

const panasonicBaseUrl = "http://%s/cgi-bin"

//PanasonicCam represents Panasonic IP cameras the TUM uses
type PanasonicCam struct {
	Ip   string
	Auth *string // currently unused as our cams have auth deactivated for PTZ operation
}

//NewPanasonicCam Acts as a constructor for cameras.
//ip: the ip address of the camera
//auth: username and password of the camera (e.g. "user:password")
func NewPanasonicCam(ip string, auth *string) *PanasonicCam {
	return &PanasonicCam{Ip: ip, Auth: auth}
}

func (c PanasonicCam) TakeSnapshot(outDir string) (filename string, err error) {
	log.Info(fmt.Sprintf("%s/view.cgi?action=snapshot", fmt.Sprintf(panasonicBaseUrl, c.Ip)))
	resp, err := makeAuthenticatedRequest(c.Auth, "GET", "", fmt.Sprintf("%s/view.cgi?action=snapshot", fmt.Sprintf(panasonicBaseUrl, c.Ip)))
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

//SetPreset tells the camera to use a preset specified by presetId
func (c PanasonicCam) SetPreset(presetId int) error {
	_, err := makeAuthenticatedRequest(c.Auth, "GET", "", fmt.Sprintf("%s/camctrl?preset=%d", fmt.Sprintf(panasonicBaseUrl, c.Ip), presetId))
	return err
}

//GetPresets fetches all presets stored on the camera
func (c PanasonicCam) GetPresets() ([]model.CameraPreset, error) {
	// panasonic cameras come with 100 slots for presets. These are always present but usually only a few are configured.
	// we therefore just return 10 stubs here.
	presets := make([]model.CameraPreset, 10)
	for i := range presets {
		presets[i].PresetID = i
		presets[i].Name = fmt.Sprintf("Preset #%2d", i)
		if i == 0 {
			presets[i].Name = "Home"
		}
	}
	return presets, nil
}
