package rest

import (
	"github.com/joschahenningsen/TUM-Live/worker/worker"
	log "github.com/sirupsen/logrus"
	"io"
	"net/http"
	"os"
)

// handleUpload handles VOD upload requests proxied by TUM-Live.
func handleUpload(w http.ResponseWriter, r *http.Request) {
	log.Info(r.URL.String())
	_, f, err := r.FormFile("file")
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
	out, err := os.CreateTemp("", "upload*"+f.Filename)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(err.Error()))
		return
	}
	_, err = io.Copy(out, inFile)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(err.Error()))
		return
	}

	streamUploadInfo, err := worker.GetStreamInfoForUploadReq(r.URL.Query().Get("key"))
	if err != nil {
		log.WithError(err).Error("Error getting stream info for upload")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(err.Error()))
		return
	}

	go worker.HandleUploadRestReq(streamUploadInfo, out.Name())
	w.WriteHeader(http.StatusOK)
}
