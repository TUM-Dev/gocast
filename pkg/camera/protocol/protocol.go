package protocol

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/icholy/digest"
)

// ErrUnexpectedStatus is returned when a camera answers with anything other than 200 OK.
// Callers that need to react to an unreachable-but-answering camera (rather than to any
// failure) can match it with errors.Is.
var ErrUnexpectedStatus = errors.New("unexpected camera response status")

// MakeAuthenticatedRequest Sends a request to the camera.
// Example usage: c.makeAuthenticatedRequest("GET", "/base","/some.cgi?preset=1")
// Returns the response body as a buffer.
//
// Anything other than 200 OK is an error: a camera that is powered down but still
// answering, one that rejects the configured credentials, or one that fails internally
// must not look like a successful preset change or a valid snapshot to the caller. The
// status code is still returned alongside the error so callers can tell "unauthorized"
// from "not found" without matching on the message.
func MakeAuthenticatedRequest(auth *string, method string, body string, url string) (*bytes.Buffer, int, error) {
	client := http.DefaultClient
	// An empty credential means no credentials are configured for this camera type
	// (camera.Service.For yields "" for a type missing from the auths map): talk to the
	// camera unauthenticated. A non-empty credential must be "user:password"; anything
	// else is a misconfiguration and is reported instead of being silently ignored.
	if auth != nil && *auth != "" {
		userPassword := strings.SplitN(*auth, ":", 2)
		if len(userPassword) != 2 {
			return nil, http.StatusBadRequest, fmt.Errorf("malformed camera credentials: expected \"user:password\"")
		}
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

	if res.StatusCode != http.StatusOK {
		// The body is not read, let alone returned: an error page is not a snapshot, and
		// handing it back is how it ended up on disk as a .jpg.
		return nil, res.StatusCode, fmt.Errorf("camera %s: %w: %s", req.URL.Host, ErrUnexpectedStatus, res.Status)
	}

	bts, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, res.StatusCode, err
	}
	return bytes.NewBuffer(bts), res.StatusCode, nil
}

// SaveResponseBuffer saves the response buffer to a file. A failed write leaves no
// file behind: a truncated snapshot on disk is indistinguishable from a whole one for
// everything downstream, so the partial file is removed instead.
func SaveResponseBuffer(outDir string, filename string, resp *bytes.Buffer) error {
	if resp == nil {
		return fmt.Errorf("no response body to save to %s", filename)
	}
	path := fmt.Sprintf("%s/%s", outDir, filename)
	imageFile, err := os.Create(path)
	if err != nil {
		return err
	}
	if _, err = imageFile.Write(resp.Bytes()); err != nil {
		_ = imageFile.Close()
		_ = os.Remove(path)
		return err
	}
	if err = imageFile.Close(); err != nil {
		_ = os.Remove(path)
		return err
	}
	return nil
}
