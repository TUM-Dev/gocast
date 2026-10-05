package apiv2

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	uuid "github.com/satori/go.uuid"
	"google.golang.org/grpc/status"

	e "github.com/TUM-Dev/gocast/apiv2/errors"
	"github.com/TUM-Dev/gocast/model"
)

// workerUploadPort is where a worker takes uploads (worker/rest), api.WorkerHTTPPort
// in v1. A variable so a test can point it at its own server.
var workerUploadPort = "8060"

var errNoWorker = errors.New("no worker is available to take the upload")

// proxyLectureMedia replaces v1's uploadVODMedia: the recording is streamed on to
// the least busy worker, which transcodes it and reports it back as the lecture's
// version of the given type. The worker learns which lecture and type through an
// upload key it trades in over gRPC, so the key is all it is sent.
//
// handleLectureUpload has authorized the caller and the lecture; nothing of the body
// has been read yet. Unlike v1, the worker is chosen before the key is created, so a
// refused upload leaves no key behind, and a lecture that is live is refused, since
// the worker's result would be discarded (NotifyUploadFinished skips live streams).
func (a *API) proxyLectureMedia(c *gin.Context, stream model.Stream) {
	videoType := model.VideoType(c.Query("type"))
	if !videoType.Valid() {
		writeUploadError(c, e.WithStatus(http.StatusBadRequest, errors.New("type must be COMB, PRES or CAM")))
		return
	}
	if stream.LiveNow {
		writeUploadError(c, e.WithStatus(http.StatusConflict, errors.New("the lecture is live")))
		return
	}

	workers := a.dao.WorkerDao.GetAliveWorkers()
	if len(workers) == 0 {
		writeUploadError(c, e.WithStatus(http.StatusServiceUnavailable, errNoWorker))
		return
	}
	worker := leastBusyWorker(workers)

	key := uuid.NewV4().String()
	if err := a.dao.UploadKeyDao.CreateUploadKey(key, stream.ID, videoType); err != nil {
		writeUploadError(c, e.WithStatus(http.StatusInternalServerError, fmt.Errorf("can't create upload key: %w", err)))
		return
	}

	target := &url.URL{
		Scheme:   "http",
		Host:     worker.Host + ":" + workerUploadPort,
		Path:     "/upload",
		RawQuery: url.Values{"key": {key}}.Encode(),
	}
	proxy := &httputil.ReverseProxy{
		Rewrite: func(r *httputil.ProxyRequest) {
			r.Out.URL = target
			r.Out.Host = target.Host
			// The worker has no use for the caller's credentials.
			r.Out.Header.Del("Authorization")
			r.Out.Header.Del("Cookie")
		},
		ModifyResponse: workerUploadResponse,
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			a.log.Warn("can't reach worker for upload", "worker", worker.WorkerID, "err", err)
			a.dropUploadKey(key)
			writeUploadError(c, e.WithStatus(http.StatusServiceUnavailable, errors.New("the worker taking the upload can't be reached")))
		},
	}
	proxy.ServeHTTP(c.Writer, c.Request)
}

// workerUploadResponse answers as the gateway answers: {} once the worker has the
// file, else an error in the gateway's shape, the worker's plain-text message in it.
// A worker's 4xx is the upload's fault (no file); anything else is the worker's.
func workerUploadResponse(resp *http.Response) error {
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4096))
	_ = resp.Body.Close()
	if err != nil {
		return err
	}

	out := []byte("{}")
	if resp.StatusCode >= 300 {
		status := http.StatusServiceUnavailable
		if resp.StatusCode >= 400 && resp.StatusCode < 500 {
			status = http.StatusBadRequest
		}
		msg := strings.TrimSpace(string(body))
		if msg == "" {
			msg = http.StatusText(resp.StatusCode)
		}
		code, jsonBody := uploadErrorBody(e.WithStatus(status, fmt.Errorf("worker: %s", msg)))
		resp.StatusCode = code
		out = jsonBody
	} else {
		resp.StatusCode = http.StatusOK
	}
	resp.Body = io.NopCloser(bytes.NewReader(out))
	resp.ContentLength = int64(len(out))
	resp.Header = http.Header{}
	resp.Header.Set("Content-Type", "application/json")
	resp.Header.Set("Content-Length", strconv.Itoa(len(out)))
	return nil
}

// uploadErrorBody is writeUploadError's answer, for a response being rewritten
// rather than written.
func uploadErrorBody(err error) (int, []byte) {
	st := status.Convert(err)
	body, _ := json.Marshal(gin.H{"code": int(st.Code()), "message": st.Message()})
	return runtime.HTTPStatusFromCode(st.Code()), body
}

// dropUploadKey deletes a key no worker will trade in. Best effort: an unused key
// grants nothing but the upload it was made for.
func (a *API) dropUploadKey(key string) {
	k, err := a.dao.UploadKeyDao.GetUploadKey(key)
	if err == nil {
		err = a.dao.UploadKeyDao.DeleteUploadKey(k)
	}
	if err != nil {
		a.log.Warn("can't delete unused upload key", "err", err)
	}
}

// leastBusyWorker is v1's getWorkerWithLeastWorkload. workers must not be empty.
func leastBusyWorker(workers []model.Worker) model.Worker {
	best := workers[0]
	for _, w := range workers[1:] {
		if w.Workload < best.Workload {
			best = w
		}
	}
	return best
}
