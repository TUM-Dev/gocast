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

type SafeStreams struct {
	mutex   sync.Mutex
	streams map[string]bool
}

var safeStreams SafeStreams

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
func (r *Runner) InitApi(addr string) {
	//http.HandleFunc("/", defaultHandler)
	http.HandleFunc("/on_publish", onPublish)
	// this endpoint should **not** be exposed to the public!
	//http.HandleFunc("/upload", handleUpload)
	log.Fatal(http.ListenAndServe(addr, nil))
}

func onPublish(w http.ResponseWriter, r *http.Request) {
	var req OnStartReq

	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, "Could not decode request", http.StatusBadRequest)
	}

	if req.Action != "publish" {
		return
	}

	streamKey, slug, err := mustGetStreamInfo(req)
	if err != nil {
		log.WithFields(log.Fields{"request": r.Form}).WithError(err).Warn("onPublish: bad request")
		http.Error(w, "Could not retrieve stream info", http.StatusBadRequest)
		return
	}

	client, err := DialIn()
	if err != nil {
		log.Error("onPublish: could not connect to server")
		return
	}

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

	go func() {
		safeStreams.mutex.Lock()
		if running, ok := safeStreams.streams[streamKey]; ok && running {
			log.Debug("onPublish: already running")
			safeStreams.mutex.Unlock()
			return
		}
		safeStreams.mutex.Unlock()
	}()
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
