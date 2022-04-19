package camera

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/joschahenningsen/TUM-Live/model"
	uuid "github.com/satori/go.uuid"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

//Camera represents AXIS IP cameras the TUM uses
type Camera struct {
	Ip   string
	Auth string
}

//NewCamera Acts as a constructor for cameras.
//ip: the ip address of the camera
//auth: username and password of the camera (e.g. "user:password")
func NewCamera(ip string, auth string) *Camera {
	return &Camera{Ip: ip, Auth: auth}
}

func (c *Camera) TakeSnapshot(outDir string) (filename string, err error) {
	filename = fmt.Sprintf("%s%s", uuid.NewV4().String(), ".jpg")
	request, err := c.makeAuthenticatedRequest("GET", "", "/axis-cgi/jpg/image.cgi?compression=75")
	if err != nil {
		return "", err
	}
	imageFile, err := os.Create(fmt.Sprintf("%s/%s", outDir, filename))
	if err != nil {
		return "", err
	}
	_, err = imageFile.Write(request.Bytes())
	if err != nil {
		return "", err
	}
	err = imageFile.Close()
	if err != nil {
		return "", err
	}
	return filename, nil
}

//SetPreset tells the camera to use a preset specified by presetId
func (c Camera) SetPreset(presetId int) error {
	_, err := c.makeAuthenticatedRequest("GET", "", fmt.Sprintf("/axis-cgi/com/ptz.cgi?gotoserverpresetno=%v&camera=1", presetId))
	if err != nil {
		return err
	}
	return nil
}

//GetPresets fetches all presets stored on the camera
func (c Camera) GetPresets() ([]model.CameraPreset, error) {
	var presetsForLectureHall []model.CameraPreset
	resp, err := c.makeAuthenticatedRequest("POST", "action=list&group=root.PTZ.Preset.P0.Position.*.Name", "/axis-cgi/param.cgi")
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

//makeAuthenticatedRequest Sends a request to the camera.
//Example usage: c.makeAuthenticatedRequest("GET", "/base","/some.cgi?preset=1")
//Returns the response body as a buffer.
func (c Camera) makeAuthenticatedRequest(method string, body string, url string) (*bytes.Buffer, error) {
	var camCurl *exec.Cmd
	switch method {
	case "GET":
		camCurl = exec.Command("curl",
			"--digest", "--user", c.Auth,
			fmt.Sprintf("http://%s%s", c.Ip, url))
	case "POST":
		camCurl = exec.Command("curl",
			"--digest", "--user", c.Auth,
			"-d", body,
			fmt.Sprintf("http://%s%s", c.Ip, url))
	default:
		return nil, fmt.Errorf("unsupported protocol: %v", method)
	}
	var bts []byte
	buf := bytes.NewBuffer(bts)
	camCurl.Stdout = buf
	err := camCurl.Start()
	if err != nil {
		return nil, err
	}
	err = camCurl.Wait()
	if err != nil {
		return nil, err
	}
	return buf, nil
}
