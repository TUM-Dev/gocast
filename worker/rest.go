package worker

import (
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
	_, err := io.WriteString(w, "Hi, I'm alive, give me some work!\n")
	if err != nil {
		http.Error(w, "Could not generate reply", http.StatusInternalServerError)
		return
	}
}

func (restRouter) liveHandler(w http.ResponseWriter, r *http.Request) {
	http.FileServer(http.Dir("/recordings")).ServeHTTP(w, r)
}
