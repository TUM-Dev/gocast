// Package rest handles notifications for self streaming from mediamtx
package rest

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/TUM-Dev/gocast/worker/cfg"
	"github.com/TUM-Dev/gocast/worker/worker"
	log "github.com/sirupsen/logrus"
)

// streams contains a map from streaming keys to their ids
var streams = safeStreams{streams: make(map[string]*worker.StreamContext)}

type safeStreams struct {
	mutex   sync.Mutex
	streams map[string]*worker.StreamContext
}

// InitApi creates routes for the api consumed by mediamtx and TUM-Live
func InitApi(addr string) {
	http.HandleFunc("/", defaultHandler)
	http.HandleFunc("/on_publish", streams.onPublish)
	// this endpoint should **not** be exposed to the public!
	http.HandleFunc("/upload", handleUpload)
	log.Fatal(http.ListenAndServe(addr, nil))
}

// mustGetStreamInfo gets the user ID and user token from mediamtx requests and verifies them with the TUM-Live API in exchange for a stream key
// The 'path' is used to indentify a user's channel without exposing his secret token to workers and ingest servers
func mustGetStreamInfo(req OnStartReq) (streamKey string, slug string, path string, err error) {
	pts := strings.Split(req.Query, "/")
	if len(pts) < 1 {
		return "", "", "", errors.New("stream key in wrong format")
	}

	token := strings.TrimPrefix(pts[0], "token=")
	if token == "" {
		return "", "", "", fmt.Errorf("missing token")
	}

	path = req.Path
	if path == "" {
		return "", "", "", errors.New("no path provided")
	}

	slug = "" // slug (e.g., 'EIDI') is optional to identify the course if the a user has overlapping lectures
	if len(pts) == 2 {
		slug = pts[1]
	}

	// TODO: Move to worker_grpc (?)
	url := fmt.Sprintf("http://%s/api/token/streamKey?slug=%s", cfg.MainBase, slug)
	request, err := http.NewRequest("POST", url, nil)
	if err != nil {
		return "", "", "", fmt.Errorf("request error: %v", err)
	}
	request.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(token+":")))

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return "", "", "", fmt.Errorf("request error: %v", err)
	}
	defer response.Body.Close()

	var apiResp struct {
		StreamKey  string `json:"stream_key"`
		StreamSlug string `json:"stream_slug"`
	}
	if err := json.NewDecoder(response.Body).Decode(&apiResp); err != nil {
		return "", "", "", fmt.Errorf("JSON decode error: %v", err)
	}
	if apiResp.StreamKey == "" {
		return "", "", "", errors.New("no stream key received from API")
	}

	return apiResp.StreamKey, apiResp.StreamSlug, path, nil
}
