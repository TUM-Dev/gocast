package camera

import (
	"errors"
	"fmt"
	"github.com/TUM-Dev/gocast/model"
	uuid "github.com/satori/go.uuid"
	"strconv"
	"strings"
)

// AxisCam represents AXIS IP cameras the TUM uses
type AxisCam struct {
	Ip   string
	Auth string
}

const axisBaseURL = "http://%s"

// NewAxisCam Acts as a constructor for cameras.
// ip: the ip address of the camera
// auth: username and password of the camera (e.g. "user:password")
func NewAxisCam(ip string, auth string) Cam {
	return &AxisCam{Ip: ip, Auth: auth}
}

func (c *AxisCam) TakeSnapshot(outDir string) (filename string, err error) {
	resp, err := makeAuthenticatedRequest(&c.Auth, "GET", "", fmt.Sprintf("%s/axis-cgi/jpg/image.cgi?compression=75", fmt.Sprintf(axisBaseURL, c.Ip)))
	if err != nil {
		return "", err
	}
	filename = fmt.Sprintf("%s%s", uuid.NewV4().String(), ".jpg")

	err = saveResponseBuffer(outDir, filename, resp)
	return filename, err
}

// SetPreset tells the camera to use a preset specified by presetId
func (c AxisCam) SetPreset(presetId int) error {
	_, err := makeAuthenticatedRequest(&c.Auth, "GET", "", fmt.Sprintf("%s/axis-cgi/com/ptz.cgi?gotoserverpresetno=%d&camera=1", fmt.Sprintf(axisBaseURL, c.Ip), presetId))
	if err != nil {
		return err
	}
	return nil
}

// GetPresets fetches all presets stored on the camera
func (c AxisCam) GetPresets() ([]model.CameraPreset, error) {
	var presetsForLectureHall []model.CameraPreset
	resp, err := makeAuthenticatedRequest(&c.Auth, "POST", "action=list&group=root.PTZ.Preset.P0.Position.*.Name", fmt.Sprintf("%s/axis-cgi/param.cgi", fmt.Sprintf(axisBaseURL, c.Ip)))
	if err != nil {
		return nil, err
	}
	body := resp.String()
	presets := strings.Split(body, "\n")
	for _, preset := range presets {
		if presetSplit := strings.Split(preset, "="); len(presetSplit) == 2 {
			idParts := strings.Split(presetSplit[0], ".")
			if len(idParts) != 7 {
				return nil, errors.New("wrong format for camera preset response")
			}
			presetId, err := strconv.ParseInt(strings.Replace(idParts[len(idParts)-2], "P", "", 1), 10, 0)
			if err != nil {
				return nil, err
			}
			presetsForLectureHall = append(presetsForLectureHall, model.CameraPreset{
				Name:     presetSplit[1],
				PresetID: int(presetId),
			})
		}
	}
	return presetsForLectureHall, nil
}
