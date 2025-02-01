package runner

import (
	"log/slog"
	"net/http"
)

type HLSServer struct {
	log *slog.Logger
	fs  http.Handler
}

func NewHLSServer(LiveDir string, log *slog.Logger) *HLSServer {
	return &HLSServer{fs: http.FileServer(http.Dir(LiveDir)), log: log}
}

func (h *HLSServer) Start() error {
	http.Handle("/", h)
	h.log.Info("starting hls server", "port", 8187)
	return http.ListenAndServe(":8187", h)
}

func (h *HLSServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	//w.Header().Set("Access-Control-Allow-Origin", "*")
	h.fs.ServeHTTP(w, r)
}
