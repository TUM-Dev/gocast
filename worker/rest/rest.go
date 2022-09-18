// Package rest handles notifications for self streaming from nginx
package rest

import (
	"errors"
	"github.com/joschahenningsen/TUM-Live/worker/cfg"
	"net/http"
	"regexp"
	"strings"
	"sync"

	"github.com/joschahenningsen/TUM-Live/worker/worker"
	log "github.com/sirupsen/logrus"
)

// streams contains a map from streaming keys to their ids
var streams = safeStreams{streams: make(map[string]*worker.StreamContext)}

type safeStreams struct {
	mutex   sync.Mutex
	streams map[string]*worker.StreamContext
}

// InitExternalApi creates routes for the hls api consumed by users or edge servers
func InitExternalApi(addr string) {
	http.HandleFunc("/hls/", http.FileServer(http.Dir(cfg.TempDir+"/hls/")).ServeHTTP)
	log.Fatal(http.ListenAndServe(addr, nil))
}

// InitInternalApi creates routes for the api consumed by nginx
func InitInternalApi(addr string) {
	http.HandleFunc("/", defaultHandler)
	http.HandleFunc("/on_publish", streams.onPublish)
	http.HandleFunc("/on_publish_done", streams.onPublishDone)
	// this endpoint should **not** be exposed!
	http.HandleFunc("/upload", handleUpload)
	log.Fatal(http.ListenAndServe(addr, nil))
}

// mustGetStreamInfo gets the stream key and slug from nginx requests and aborts with bad request if something is wrong
func mustGetStreamInfo(r *http.Request) (streamKey string, slug string, err error) {
	name := r.FormValue("name")
	if name == "" {
		return "", "", errors.New("no stream slug")
	}
	tcUrl := r.FormValue("tcurl")
	if tcUrl == "" {
		return "", "", errors.New("no stream key")
	}
	if m, _ := regexp.MatchString(".+\\?secret=.+", tcUrl); !m {
		return "", "", errors.New("stream key invalid")
	}
	key := strings.Split(tcUrl, "?secret=")[1]
	return key, name, nil
}
