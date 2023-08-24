// Package rest handles notifications for self streaming from mediamtx
package rest

import (
	"errors"
	"net/http"
	"strings"
	"sync"

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

// mustGetStreamInfo gets the stream key and slug from mediamtx requests and aborts with bad request if something is wrong
func mustGetStreamInfo(req OnStartReq) (streamKey string, slug string, err error) {
	pts := strings.Split(req.Query, "/")
	if len(pts) != 2 {
		return "", "", errors.New("stream key in wrong format")
	}
	key := strings.TrimPrefix(pts[0], "secret=")
	if key == "" {
		return "", "", errors.New("no stream key provided")
	}
	slug = pts[1]
	if slug == "" {
		return "", "", errors.New("no slug provided")
	}
	return key, slug, nil
}
