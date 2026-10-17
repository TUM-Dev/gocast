package apiv2

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc/status"

	e "github.com/TUM-Dev/gocast/apiv2/errors"
	"github.com/TUM-Dev/gocast/model"
)

var errNoRunner = errors.New("no runner is available to take the upload")

// UploadTarget is where a runner takes one upload.
type UploadTarget struct {
	Host string
	Port int
	// Token is single use and tells the runner which lecture the file belongs to.
	Token string
}

// Uploads reserves a runner for a lecture's uploaded recording. Only a runner converts
// uploads, so without one (see WithUploads) uploading answers 503.
type Uploads interface {
	// Reserve picks a runner and returns where to send the file for the given version of
	// the stream. The runner learns which stream and version through the token.
	Reserve(ctx context.Context, streamID uint, version model.StreamVersion) (UploadTarget, error)
}

// WithUploads lets uploading a lecture's recording reach a runner. Without it the
// upload answers 503.
func WithUploads(u Uploads) Option {
	return func(a *API) { a.uploads = u }
}

// proxyLectureMedia replaces v1's uploadVODMedia: the recording is streamed on to a
// runner, which converts it and reports it back as the lecture's version of the given
// type. The runner learns which lecture and type through a token it handed out when it
// was reserved, so the token is all the caller sends it.
//
// handleLectureUpload has authorized the caller and the lecture; nothing of the body
// has been read yet. A runner is reserved before the body is touched, so a refused
// upload costs nothing, and a lecture that is live is refused, since the VOD of its own
// stream would race the upload for the same playlist.
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
	if a.uploads == nil {
		writeUploadError(c, e.WithStatus(http.StatusServiceUnavailable, errNoRunner))
		return
	}

	slot, err := a.uploads.Reserve(c.Request.Context(), stream.ID, model.StreamVersion(videoType))
	if err != nil {
		a.log.Warn("can't reserve a runner for an upload", "stream", stream.ID, "err", err)
		writeUploadError(c, e.WithStatus(http.StatusServiceUnavailable, errNoRunner))
		return
	}

	// The card shows the upload as converting from here on. The runner's reports move the
	// percentage, VODReady removes the row and VODFailed marks it failed. Replacing the row
	// also clears a failure left by an earlier upload.
	version := model.StreamVersion(videoType)
	// What the row said before, for when the runner doesn't take this file after all: it
	// may be another upload still converting, or a failure its uploader hasn't seen yet.
	previous, err := a.dao.StreamsDao.GetTranscodingProgressByVersion(version, stream.ID)
	hadPrevious := err == nil
	if err := a.dao.StreamsDao.SaveTranscodingProgress(model.TranscodingProgress{StreamID: stream.ID, Version: version}); err != nil {
		// Not worth refusing the upload: the video is made all the same, the card just
		// doesn't show it converting.
		a.log.Warn("can't mark the upload as converting", "stream", stream.ID, "err", err)
	}
	// If the runner doesn't take the file there is no conversion of this upload to show.
	dropProgress := func() {
		var err error
		if hadPrevious {
			err = a.dao.StreamsDao.SaveTranscodingProgress(previous)
		} else {
			err = a.dao.StreamsDao.RemoveTranscodingProgress(version, stream.ID)
		}
		if err != nil {
			a.log.Warn("can't clear the progress of an upload that wasn't taken", "stream", stream.ID, "err", err)
		}
	}
	target := &url.URL{
		Scheme: "http",
		Host:   net.JoinHostPort(slot.Host, strconv.Itoa(slot.Port)),
		Path:   "/upload/" + slot.Token,
	}
	proxy := &httputil.ReverseProxy{
		Rewrite: func(r *httputil.ProxyRequest) {
			r.Out.URL = target
			r.Out.Host = target.Host
			// The runner has no use for the caller's credentials.
			r.Out.Header.Del("Authorization")
			r.Out.Header.Del("Cookie")
		},
		ModifyResponse: func(resp *http.Response) error {
			err := runnerUploadResponse(resp)
			if resp.StatusCode != http.StatusOK {
				dropProgress()
			}
			return err
		},
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			a.log.Warn("can't reach runner for upload", "runner", slot.Host, "err", err)
			dropProgress()
			writeUploadError(c, e.WithStatus(http.StatusServiceUnavailable, errors.New("the runner taking the upload can't be reached")))
		},
	}
	proxy.ServeHTTP(c.Writer, c.Request)
}

// runnerUploadResponse answers as the gateway answers: {} once the runner has the
// file, else an error in the gateway's shape, the runner's plain-text message in it.
// A runner's 4xx is the upload's fault (no file), except that it lost the token; anything
// else is the runner's.
func runnerUploadResponse(resp *http.Response) error {
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4096))
	_ = resp.Body.Close()
	if err != nil {
		return err
	}

	out := []byte("{}")
	if resp.StatusCode >= 300 {
		status := http.StatusServiceUnavailable
		// A 404 is the runner not knowing the token it handed out a moment ago, because it
		// restarted or took too long: the upload is worth retrying, unlike one without a file.
		if resp.StatusCode >= 400 && resp.StatusCode < 500 && resp.StatusCode != http.StatusNotFound {
			status = http.StatusBadRequest
		}
		msg := strings.TrimSpace(string(body))
		if msg == "" {
			msg = http.StatusText(resp.StatusCode)
		}
		code, jsonBody := uploadErrorBody(e.WithStatus(status, fmt.Errorf("runner: %s", msg)))
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
