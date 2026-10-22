package apiv2

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

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
func (a *API) handleSelfStreamPublish(c *gin.Context) {
	var req onStartReq
	if err := json.NewDecoder(c.Request.Body).Decode(&req); err != nil {
		a.log.Warn("Bad request when trying to onPublish")
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	if req.Action != "publish" {
		return
	}

	streamKey, slug, err := mustGetStreamInfo(req)
	if err != nil {
		a.log.With("request", c.Request.Form).With("err", err).Warn("onPublish: bad request")
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	stream, err := a.dao.StreamsDao.GetStreamByKey(c, streamKey)
	if err != nil {
		a.log.Error("onPublish: failed to get stream key")
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	course, err := a.dao.CoursesDao.GetCourseById(c, stream.CourseID)
	if err != nil {
		a.log.Error("onPublish: failed to get course id")
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	if slug != fmt.Sprintf("%s-%d", course.Slug, stream.ID) {
		a.log.Error("onPublish: slug mismatch")
		c.AbortWithStatus(http.StatusForbidden)
		return
	}

	if stream.LiveNow {
		a.log.Warn("onPublish: stream is already live")
	}

	if a.selfStreams == nil {
		a.log.Error("onPublish: no runner manager configured")
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	if err := a.selfStreams.RequestSelfStream(c, stream); err != nil {
		a.log.Error("Failed to start stream on runners")
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	c.Status(http.StatusOK)
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
