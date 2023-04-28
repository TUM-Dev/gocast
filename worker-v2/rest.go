package worker

import (
	"github.com/joschahenningsen/TUM-Live/worker/cfg"
	"io"
	"net/http"
)

type restRouter struct {
}

// defaultHandler tells that the current worker is active and has a valid ID
func (restRouter) defaultHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
	if cfg.WorkerID == "" {
		http.Error(w, "Worker has no ID", http.StatusInternalServerError)
		return
	}
	_, err := io.WriteString(w, "Hi, I'm alive, give me some work!\n")
	if err != nil {
		http.Error(w, "Could not generate reply", http.StatusInternalServerError)
		return
	}
}
