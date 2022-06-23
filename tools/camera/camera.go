package camera

import (
	"bytes"
	"fmt"
	"github.com/icholy/digest"
	"github.com/joschahenningsen/TUM-Live/model"
	"io"
	"net/http"
	"os"
	"strings"
)

type Cam interface {
	// SetPreset moves the camera to the preset identified by preset.
	SetPreset(presetId int) error
	// TakeSnapshot creates a snapshot and returns the filename of it.
	TakeSnapshot(outDir string) (filename string, err error)
	// GetPresets fetches all available presets
	GetPresets() ([]model.CameraPreset, error)
}

//makeAuthenticatedRequest Sends a request to the camera.
//Example usage: c.makeAuthenticatedRequest("GET", "/base","/some.cgi?preset=1")
//Returns the response body as a buffer.
func makeAuthenticatedRequest(auth *string, method string, body string, url string) (*bytes.Buffer, error) {
	// var camCurl *exec.Cmd
	client := http.DefaultClient
	if auth != nil {
		userPassword := strings.Split(*auth, ":")
		client = &http.Client{
			Transport: &digest.Transport{
				Username: userPassword[0],
				Password: userPassword[1],
			},
		}
	}

	var req *http.Request
	var err error
	switch method {
	case "GET":
		req, err = http.NewRequest("GET", url, nil)
	case "POST":
		req, err = http.NewRequest("POST", url, bytes.NewReader([]byte(body)))
	default:
		return nil, fmt.Errorf("unsupported protocol: %v", method)
	}
	if err != nil {
		return nil, fmt.Errorf("create http request: %v", err)
	}

	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	bts, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	return bytes.NewBuffer(bts), nil
}

func saveResponseBuffer(outDir string, filename string, resp *bytes.Buffer) error {
	imageFile, err := os.Create(fmt.Sprintf("%s/%s", outDir, filename))
	if err != nil {
		return err
	}
	_, err = imageFile.Write(resp.Bytes())
	if err != nil {
		return err
	}
	err = imageFile.Close()
	if err != nil {
		return err
	}
	return nil
}
