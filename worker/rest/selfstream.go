package rest

import (
	"fmt"
	"google.golang.org/grpc"
	"io"
	"net/http"
	"strings"

	"github.com/joschahenningsen/TUM-Live/worker/cfg"
	"github.com/joschahenningsen/TUM-Live/worker/pb"
	"github.com/joschahenningsen/TUM-Live/worker/worker"
	log "github.com/sirupsen/logrus"
)

// defaultHandler tells that the current worker is active and has a valid ID
func defaultHandler(w http.ResponseWriter, r *http.Request) {
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

// onPublishDone is called by nginx when the stream stops publishing
func (s *safeStreams) onPublishDone(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	log.Info("onPublishDone called")
	streamKey, _, err := mustGetStreamInfo(r)
	if err != nil {
		log.WithFields(log.Fields{"request": r.Form}).WithError(err).Warn("onPublishDone: bad request")
		http.Error(w, "Could not retrieve stream info", http.StatusBadRequest)
		return
	}
	s.mutex.Lock()
	defer s.mutex.Unlock()
	if streamCtx, ok := s.streams[streamKey]; ok {
		go func() {
			worker.HandleStreamEnd(streamCtx, false)
			worker.NotifyStreamDone(streamCtx)
			worker.HandleSelfStreamRecordEnd(streamCtx)
		}()
	} else {
		errorText := "stream key not existing in self streams map"
		log.WithField("streamKey", streamKey).Error(errorText)
		http.Error(w, errorText, http.StatusBadRequest)
	}
}

// formatRequest generates ascii representation of a request
func formatRequest(r *http.Request) string {
	// Create return string
	var request []string
	url := fmt.Sprintf("%v %v %v", r.Method, r.URL, r.Proto)
	request = append(request, url)
	request = append(request, fmt.Sprintf("Host: %v", r.Host))
	request = append(request, "remote addr: "+r.RemoteAddr)
	for name, headers := range r.Header {
		name = strings.ToLower(name)
		for _, h := range headers {
			request = append(request, fmt.Sprintf("%v: %v", name, h))
		}
	}

	// If this is a POST, add post data
	if r.Method == "POST" {
		r.ParseForm()
		request = append(request, "\n")
		request = append(request, r.Form.Encode())
	}
	return strings.Join(request, "\n")
}

// onPublish is called by nginx when the stream starts publishing
func (s *safeStreams) onPublish(w http.ResponseWriter, r *http.Request) {
	log.Info("onPublish called")
	fmt.Println(formatRequest(r))
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	streamKey, slug, err := mustGetStreamInfo(r)
	if err != nil {
		log.WithFields(log.Fields{"request": r.Form}).WithError(err).Warn("onPublish: bad request")
		http.Error(w, "Could not retrieve stream info", http.StatusBadRequest)
		return
	}
	client, conn, err := worker.GetClient()

	defer func(conn *grpc.ClientConn) {
		if err := conn.Close(); err != nil {
			log.WithFields(log.Fields{"request": r.Form}).WithError(err).Warn("Could not connect to client")
		}
	}(conn)

	if err != nil {
		http.Error(w, "Could not establish connection to client", http.StatusInternalServerError)
		return
	}
	resp, err := client.SendSelfStreamRequest(r.Context(), &pb.SelfStreamRequest{
		WorkerID:   cfg.WorkerID,
		StreamKey:  streamKey,
		CourseSlug: slug,
	})
	if err != nil {
		http.Error(w, "Authentication failed for SendSelfStreamRequest", http.StatusForbidden)
		return
	}
	s.mutex.Lock()
	if streamCtx, ok := s.streams[streamKey]; ok {
		log.Debug("SelfStream already exists, stopping it.")
		worker.HandleStreamEnd(streamCtx, true)
	}
	s.mutex.Unlock()

	// register stream in local map
	streamContext := worker.HandleSelfStream(resp, slug)

	s.mutex.Lock()
	s.streams[streamKey] = streamContext
	s.mutex.Unlock()
}
