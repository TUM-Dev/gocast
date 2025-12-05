package protocol

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/icholy/digest"
)

// MakeAuthenticatedRequest Sends a request to the camera.
// Example usage: c.makeAuthenticatedRequest("GET", "/base","/some.cgi?preset=1")
// Returns the response body as a buffer.
func MakeAuthenticatedRequest(auth *string, method string, body string, url string) (*bytes.Buffer, int, error) {
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
		return nil, http.StatusBadRequest, fmt.Errorf("unsupported protocol: %v", method)
	}
	if err != nil {
		return nil, http.StatusBadRequest, fmt.Errorf("create http request: %v", err)
	}

	res, err := client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer func() {
		_ = res.Body.Close()
	}()

	bts, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, res.StatusCode, err
	}
	return bytes.NewBuffer(bts), res.StatusCode, nil
}

// SaveResponseBuffer saves the response buffer to a file
func SaveResponseBuffer(outDir string, filename string, resp *bytes.Buffer) error {
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
