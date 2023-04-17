package rest

import (
	"encoding/json"
	"fmt"
	"google.golang.org/grpc"
	"io"
	"net/http"

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
	var req OnStartReq
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		fmt.Println(err)
		http.Error(w, "Could not decode request", http.StatusBadRequest)
		return
	}
	streamKey, _, err := mustGetStreamInfo(req)
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

// todo remove this
// onPublish is called by nginx when the stream starts publishing
func (s *safeStreams) onPublish(w http.ResponseWriter, r *http.Request) {
	log.Info("onPublish called")
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req OnStartReq
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, "Could not decode request", http.StatusBadRequest)
		return
	}
	if req.Action != "publish" {
		// all good, client is reading
		return
	}
	streamKey, slug, err := mustGetStreamInfo(req)
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
		log.Error(err)
		http.Error(w, "Authentication failed for SendSelfStreamRequest", http.StatusForbidden)
		return
	}
	s.mutex.Lock()
	if streamCtx, ok := s.streams[streamKey]; ok {
		log.Debug("SelfStream already exists, stopping it.")
		worker.HandleStreamEnd(streamCtx, true)
	}
	s.mutex.Unlock()

	// todo is this right?
	// register stream in local map
	streamContext := worker.HandleSelfStream(resp, slug)

	s.mutex.Lock()
	s.streams[streamKey] = streamContext // todo this is only added after the stream has ended
	s.mutex.Unlock()

	go func() {
		worker.HandleStreamEnd(streamContext, false)
		worker.NotifyStreamDone(streamContext)
		worker.HandleSelfStreamRecordEnd(streamContext)
	}()
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
