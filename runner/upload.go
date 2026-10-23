package runner

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/tum-dev/gocast/runner/config"
	"github.com/tum-dev/gocast/runner/pkg/actions"
)

// uploadTokenTTL is how long a PrepareUpload token can be traded in. The server
// uses it right away, so this only has to cover the time to connect.
const uploadTokenTTL = 10 * time.Minute

// importActions is what an upload runs. The thumbnail is made from the VOD that
// ImportVOD leaves, see its playlistURL. A variable so the tests can swap the import out.
var importActions = []actions.Action{actions.ImportVOD, actions.RegenerateThumb}

type uploadSlot struct {
	streamID uint64
	version  string
	expires  time.Time
}

// uploadSlots hands out the single-use tokens the upload endpoint accepts. The endpoint
// has no other authentication: it must only be reachable by the server, which gets a
// token over gRPC for every upload it proxies.
type uploadSlots struct {
	mu    sync.Mutex
	slots map[string]uploadSlot
	now   func() time.Time
}

func newUploadSlots() *uploadSlots {
	return &uploadSlots{slots: make(map[string]uploadSlot), now: time.Now}
}

func (u *uploadSlots) create(streamID uint64, version string) (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate upload token: %w", err)
	}
	token := hex.EncodeToString(b)

	u.mu.Lock()
	defer u.mu.Unlock()
	// Tokens that were never used would pile up otherwise.
	for t, s := range u.slots {
		if !s.expires.After(u.now()) {
			delete(u.slots, t)
		}
	}
	u.slots[token] = uploadSlot{streamID: streamID, version: version, expires: u.now().Add(uploadTokenTTL)}
	return token, nil
}

// take redeems a token. It is consumed even if the upload then fails: the server asks
// for a new one with every upload attempt, and a token that can't be redeemed twice
// can't be raced.
func (u *uploadSlots) take(token string) (uploadSlot, bool) {
	u.mu.Lock()
	defer u.mu.Unlock()
	s, ok := u.slots[token]
	delete(u.slots, token)
	if !ok || !s.expires.After(u.now()) {
		return uploadSlot{}, false
	}
	return s, true
}

// serveUploads runs the HTTP endpoint videos are uploaded to until ctx ends. It has its
// own mux because the HLS server registers on the default one.
func (r *Runner) serveUploads(ctx context.Context) {
	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", config.Config.UploadPort),
		Handler: r.uploadHandler(),
		// No read timeout: uploads are large. Slow bodies are the server's proxy to cut.
		ReadHeaderTimeout: 10 * time.Second,
	}
	go func() {
		<-ctx.Done()
		_ = srv.Close()
	}()
	r.log.Info("starting upload server", "port", config.Config.UploadPort)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		r.log.Error("error starting upload server", "error", err)
	}
}

func (r *Runner) uploadHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /upload/{token}", r.handleUpload)
	return mux
}

// handleUpload stores the file of the multipart body and starts the import. As with the
// worker's upload endpoint it answers once the file is on disk, not once it is converted.
func (r *Runner) handleUpload(w http.ResponseWriter, req *http.Request) {
	// A runner that is draining must not exit while the body is still arriving. The job
	// RunAction creates holds the runner itself, and it takes that hold before returning,
	// so there is no gap between this hold and that one.
	defer r.hold()()

	slot, ok := r.uploads.take(req.PathValue("token"))
	if !ok {
		http.Error(w, "unknown or expired upload token", http.StatusNotFound)
		return
	}
	log := r.log.With("stream_id", slot.streamID, "stream_version", slot.version)

	// Streamed part by part: ParseMultipartForm would spool the whole video through memory and a second temp file.
	mr, err := req.MultipartReader()
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	var file string
	for {
		part, err := mr.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if part.FormName() != "file" {
			continue
		}
		var size int64
		file, size, err = r.storeUpload(part)
		if err != nil {
			log.Error("storing upload failed", "error", err)
			http.Error(w, "couldn't store the upload", http.StatusInternalServerError)
			return
		}
		if size == 0 {
			// Answering 200 would send the uploader off to wait for a video that
			// the import can only fail to make.
			_ = os.Remove(file)
			http.Error(w, "the file is empty", http.StatusBadRequest)
			return
		}
		break
	}
	if file == "" {
		http.Error(w, "no file in request", http.StatusBadRequest)
		return
	}

	data := map[string]any{
		"streamID":      slot.streamID,
		"streamVersion": slot.version,
		"uploadPath":    file,
	}
	jID := r.RunAction(importActions, nil, data, log)
	log.Info("upload import job added", "ID", jID)
	w.WriteHeader(http.StatusOK)
}

func (r *Runner) storeUpload(src io.Reader) (string, int64, error) {
	if err := os.MkdirAll(config.Config.UploadPath, os.ModePerm); err != nil {
		return "", 0, err
	}
	// The uploader's file name is not used: ffmpeg probes the content.
	out, err := os.CreateTemp(config.Config.UploadPath, "upload-*")
	if err != nil {
		return "", 0, err
	}
	n, err := io.Copy(out, src)
	if err != nil {
		_ = out.Close()
		_ = os.Remove(out.Name())
		return "", 0, err
	}
	if err := out.Close(); err != nil {
		_ = os.Remove(out.Name())
		return "", 0, err
	}
	return out.Name(), n, nil
}
