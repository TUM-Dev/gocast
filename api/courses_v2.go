package api

import (
	"errors"
	"github.com/getsentry/sentry-go"
	"github.com/gin-gonic/gin"
	"github.com/joschahenningsen/TUM-Live/dao"
	"github.com/joschahenningsen/TUM-Live/model"
	"github.com/joschahenningsen/TUM-Live/tools"
	"gorm.io/gorm"
	"net/http"
)

func NewCoursesV2Router(router *gin.RouterGroup, daoWrapper dao.DaoWrapper) {
	routes := coursesV2Routes{daoWrapper}

	courseById := router.Group("/courses/:id")
	{
		courseById.GET("", routes.getCourse)
	}
}

type coursesV2Routes struct {
	dao.DaoWrapper
}

type coursesByIdURI struct {
	ID uint `uri:"id" binding:"required"`
}

func (r coursesV2Routes) getCourse(c *gin.Context) {
	tumLiveContext := c.MustGet("TUMLiveContext").(tools.TUMLiveContext)

	var uri coursesByIdURI
	if err := c.ShouldBindUri(&uri); err != nil {
		_ = c.Error(tools.RequestError{
			Err:           err,
			Status:        http.StatusBadRequest,
			CustomMessage: "invalid URI",
		})
		return
	}

	// watchedStateData is used by the client to track the which VoDs are watched.
	type watchedStateData struct {
		ID        uint   `json:"streamID"`
		Month     string `json:"month"`
		Watched   bool   `json:"watched"`
		Recording bool   `json:"recording"`
	}

	type Response struct {
		Course       model.Course
		WatchedState []watchedStateData `json:",omitempty"`
	}

	course, err := r.CoursesDao.GetCourseById(c, uri.ID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			_ = c.Error(tools.RequestError{
				Status:        http.StatusBadRequest,
				CustomMessage: "can't find course",
			})
		} else {
			sentry.CaptureException(err)
			_ = c.Error(tools.RequestError{
				Err:           err,
				Status:        http.StatusInternalServerError,
				CustomMessage: "can't retrieve course",
			})
		}
		return
	}
	var response Response
	if tumLiveContext.User == nil {
		// Not-Logged-In Users do not receive the watch state
		response = Response{Course: course}
	} else {
		streamsWithWatchState, err := r.StreamsDao.GetStreamsWithWatchState(course.ID, (*tumLiveContext.User).ID)
		if err != nil {
			sentry.CaptureException(err)
			_ = c.Error(tools.RequestError{
				Err:           err,
				Status:        http.StatusInternalServerError,
				CustomMessage: "loading streamsWithWatchState and progresses for a given course and user failed",
			})
		}

		course.Streams = streamsWithWatchState // Update the course streams to contain the watch state.

		var clientWatchState = make([]watchedStateData, 0)
		for _, s := range streamsWithWatchState {
			clientWatchState = append(clientWatchState, watchedStateData{
				ID:        s.Model.ID,
				Month:     s.Start.Month().String(),
				Watched:   s.Watched,
				Recording: s.Recording,
			})
		}

		response = Response{Course: course, WatchedState: clientWatchState}
	}

	c.JSON(http.StatusOK, response)
}
