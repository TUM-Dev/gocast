package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/pkg/runner_manager"
	"github.com/gin-gonic/gin"
)

type selfstreamRoutes struct {
	daoWrapper dao.DaoWrapper
	manager    *runner_manager.Manager
}

type SafeStreamStructStream struct {
	Ip        string
	StreamKey string
	Slug      string
	Retries   int
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

func configSelfstreamRouter(router *gin.Engine, daoWrapper dao.DaoWrapper, manager *runner_manager.Manager) {
	routes := selfstreamRoutes{
		daoWrapper: daoWrapper,
		manager:    manager,
	}
	router.POST("/api/selfstream/onPublish", routes.onPublish)
}

func (r *selfstreamRoutes) onPublish(c *gin.Context) {
	var req OnStartReq

	err := json.NewDecoder(c.Request.Body).Decode(&req)
	if err != nil {
		err = c.AbortWithError(http.StatusBadRequest, errors.New("could not decode request"))
		if err != nil {
			logger.Error("Error when aborting request")
		}
		return
	}

	if req.Action != "publish" {
		return
	}

	streamKey, slug, err := mustGetStreamInfo(req)
	if err != nil {
		logger.With("request", c.Request.Form).With("err", err).Warn("onPublish: bad request")
		err = c.AbortWithError(http.StatusBadRequest, errors.New("could not retrieve stream info"))
		if err != nil {
			logger.Error("Error when aborting request")
		}
		return
	}

	stream, err := r.daoWrapper.StreamsDao.GetStreamByKey(c, streamKey)
	if err != nil {
		logger.Error("onPublish: failed to get stream key")
		err = c.AbortWithError(http.StatusInternalServerError, errors.New("could not retrieve stream key"))
		if err != nil {
			logger.Error("Error when aborting request")
		}
		return
	}

	course, err := r.daoWrapper.CoursesDao.GetCourseById(c, stream.CourseID)
	if err != nil {
		logger.Error("onPublish: failed to get course id")
		err = c.AbortWithError(http.StatusInternalServerError, errors.New("could not retrieve course id"))
		if err != nil {
			logger.Error("Error when aborting request")
		}
		return
	}

	if slug != fmt.Sprintf("%s-%d", course.Slug, stream.ID) {
		logger.Error("onPublish: slug mismatch")
		err = c.AbortWithError(http.StatusForbidden, errors.New("authentication failed"))
		if err != nil {
			logger.Error("Error when aborting request")
		}
		return
	}

	if stream.LiveNow {
		logger.Warn("onPublish: stream is already live")
	}

	err = r.manager.RequestSelfStream(c, stream)
	if err != nil {
		logger.Error("Failed to start stream on runners")
		err = c.AbortWithError(http.StatusInternalServerError, errors.New("Failed to request stream"))
		if err != nil {
			logger.Error("Error when aborting request")
		}
		return
	}
	c.Status(http.StatusOK)
}

// mustGetStreamInfo gets the stream key and slug from mediamtx requests and aborts with bad request if something is wrong
func mustGetStreamInfo(req OnStartReq) (streamKey string, slug string, err error) {
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
