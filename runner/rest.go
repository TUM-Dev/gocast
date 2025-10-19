package runner

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"sync"

	log "github.com/sirupsen/logrus"
	"github.com/tum-dev/gocast/runner/config"
	"github.com/tum-dev/gocast/runner/protobuf"
)

type SafeStreamStructStream struct {
	Ip        string
	StreamKey string
	Slug      string
	Retries   int
}

type SafeStreamStruct struct {
	mutex   sync.Mutex
	streams map[string]SafeStreamStructStream
}

var SafeStreams = SafeStreamStruct{
	mutex:   sync.Mutex{},
	streams: make(map[string]SafeStreamStructStream),
}

type OnStartReq struct {
	Ip       string `json:"ip"`
	User     string `json:"user"`
	Password string `json:"password"`
	Path     string `json:"path"`
	Protocol string `json:"protocol"`
	Id       string `json:"id"`
	Action   string `json:"action"`
	Query    string `json:"query"`
}

// InitApi creates routes for the api consumed by mediamtx and TUM-Live
// TODO: Move this to TUM-Live, so everything is handeled by the main instance
func (r *Runner) InitApi(addr string) {
	http.HandleFunc("/on_publish", r.onPublish)
	// this endpoint should **not** be exposed to the public!
	// TODO: Add Upload endpoint
	log.Fatal(http.ListenAndServe(addr, nil))
}

func (r *Runner) onPublish(w http.ResponseWriter, request *http.Request) {
	if request.Method != "POST" {
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
	var req OnStartReq

	err := json.NewDecoder(request.Body).Decode(&req)
	if err != nil {
		http.Error(w, "Could not decode request", http.StatusBadRequest)
	}

	if req.Action != "publish" {
		return
	}

	streamKey, slug, err := mustGetStreamInfo(req)
	if err != nil {
		log.WithFields(log.Fields{"request": request.Form}).WithError(err).Warn("onPublish: bad request")
		http.Error(w, "Could not retrieve stream info", http.StatusBadRequest)
		return
	}

	client, err := r.dialIn()
	if err != nil {
		log.Error("onPublish: could not connect to server")
		return
	}

	SafeStreams.mutex.Lock()
	if safeStreamStruct, ok := SafeStreams.streams[slug]; ok && safeStreamStruct.StreamKey == streamKey {
		log.Debug("onPublish: already running")
		SafeStreams.mutex.Unlock()
		return
	}
	SafeStreams.streams[slug] = SafeStreamStructStream{
		Ip:        req.Ip,
		StreamKey: streamKey,
		Slug:      slug,
		Retries:   0,
	}
	SafeStreams.mutex.Unlock()

	_, err = client.RequestSelfStream(context.Background(), &protobuf.SelfStreamRequest{
		Hostname:   &config.Config.Hostname,
		StreamKey:  &streamKey,
		CourseSlug: &slug,
	})
	if err != nil {
		log.Error(err)
		http.Error(w, "Authentication failed for SendSelfStreamRequest", http.StatusForbidden)
		return
	}
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
