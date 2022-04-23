package rest

import (
	"fmt"
	"github.com/joschahenningsen/TUM-Live/worker/worker"
	log "github.com/sirupsen/logrus"
	"io"
	"io/ioutil"
	"net/http"
)

// handleUpload handles VOD upload requests proxied by TUM-Live.
func handleUpload(w http.ResponseWriter, r *http.Request) {
	log.Info(r.URL.String())
	_, f, err := r.FormFile("video")
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(err.Error()))
		return
	}
	inFile, err := f.Open()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(err.Error()))
		return
	}
	out, err := ioutil.TempFile("", "upload*"+f.Filename)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(err.Error()))
		return
	}
	written, err := io.Copy(out, inFile)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(err.Error()))
		return
	}
	log.Info("Video file written to disk (", fmt.Sprintf("%d", written)+" bytes)"+out.Name())
	worker.HandleUploadRestReq(r.URL.Query().Get("key"), out.Name())
}
