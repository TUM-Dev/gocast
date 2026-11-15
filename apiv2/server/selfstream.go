package apiv2

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/TUM-Dev/gocast/model"
)

// SelfStreamStarter asks the runners to take a stream that a lecturer publishes
// themselves; *runner_manager.Manager satisfies it.
type SelfStreamStarter interface {
	RequestSelfStream(ctx context.Context, stream model.Stream) error
}

// WithSelfStreamStarter gives the API the means to start a self stream. Without one,
// the publish webhook answers 500.
func WithSelfStreamStarter(s SelfStreamStarter) Option {
	return func(a *API) { a.selfStreams = s }
}

// onStartReq is what mediamtx posts to its authHTTPAddress.
type onStartReq struct {
	Ip       string `json:"ip"`
	User     string `json:"user"`
	Password string `json:"password"`
	Path     string `json:"path"`
	Protocol string `json:"protocol"`
	Id       string `json:"id"`
	Action   string `json:"action"`
	Query    string `json:"query"`
}

// handleSelfStreamPublish is the mediamtx ingest server's authHTTPAddress webhook
// (ingest/mediamtx.yml), asked to approve every RTMP publish: a 2xx lets it through.
// Any action other than "publish" is approved without a check, as in v1.
func (a *API) handleSelfStreamPublish(w http.ResponseWriter, r *http.Request) {
	var req onStartReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		a.log.Warn("Bad request when trying to onPublish")
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if req.Action != "publish" {
		return
	}

	streamKey, slug, err := mustGetStreamInfo(req)
	if err != nil {
		a.log.With("request", r.Form).With("err", err).Warn("onPublish: bad request")
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	stream, err := a.dao.StreamsDao.GetStreamByKey(r.Context(), streamKey)
	if err != nil {
		a.log.Error("onPublish: failed to get stream key")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	course, err := a.dao.CoursesDao.GetCourseById(r.Context(), stream.CourseID)
	if err != nil {
		a.log.Error("onPublish: failed to get course id")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if slug != fmt.Sprintf("%s-%d", course.Slug, stream.ID) {
		a.log.Error("onPublish: slug mismatch")
		w.WriteHeader(http.StatusForbidden)
		return
	}

	if stream.LiveNow {
		a.log.Warn("onPublish: stream is already live")
	}

	if a.selfStreams == nil {
		a.log.Error("onPublish: no runner manager configured")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	// Not the request's context: the caller may hang up before the runners answer, and
	// a stream half-started because of it is worse than one started for nobody.
	if err := a.selfStreams.RequestSelfStream(context.WithoutCancel(r.Context()), stream); err != nil {
		a.log.Error("Failed to start stream on runners")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// mustGetStreamInfo gets the stream key and slug from mediamtx requests; the error says what is wrong.
func mustGetStreamInfo(req onStartReq) (streamKey string, slug string, err error) {
	pts := strings.Split(req.Query, "/")
	if len(pts) != 2 {
		return "", "", errors.New("stream key in wrong format")
	}
	key := strings.TrimPrefix(pts[0], "secret=")
	if key == "" {
		return "", "", errors.New("no stream key provided")
	}
	slug = pts[1]
	if slug == "" {
		return "", "", errors.New("no slug provided")
	}
	return key, slug, nil
}
