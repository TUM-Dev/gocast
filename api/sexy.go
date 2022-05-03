package api

import (
	"github.com/gin-gonic/gin"
	"github.com/joschahenningsen/TUM-Live/dao"
	"net/http"
	"time"
)

func configGinSexyApiRouter(router gin.IRoutes, daoWrapper dao.DaoWrapper) {
	routes := sexyRoutes{daoWrapper}
	router.GET("/api/sexy", routes.getStreamInfo)
}

type sexyRoutes struct {
	dao.DaoWrapper
}

func (r sexyRoutes) getStreamInfo(context *gin.Context) {
	courses, err := r.CoursesDao.GetAllCourses()
	if err != nil {
		context.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"status": "something went wrong"})
		return
	}
	var resp []course
	for _, c := range courses {
		if c.Visibility != "public" {
			continue
		}
		currentCourse := course{c.Name, []stream{}}
		for _, s := range c.Streams {
			if !s.LiveNow && !s.Recording {
				continue
			}
			var sources []string
			if s.PlaylistUrl != "" {
				sources = append(sources, s.PlaylistUrl)
			}
			if s.PlaylistUrlPRES != "" {
				sources = append(sources, s.PlaylistUrlPRES)
			}
			if s.PlaylistUrlCAM != "" {
				sources = append(sources, s.PlaylistUrlCAM)
			}
			currentCourse.Streams = append(currentCourse.Streams, stream{
				StreamName: s.Name,
				Start:      s.Start,
				End:        s.End,
				Sources:    sources,
				Live:       s.LiveNow,
			})
		}
		resp = append(resp, currentCourse)
	}
	context.JSON(http.StatusOK, resp)
}

type course struct {
	CourseName string   `json:"course_name"`
	Streams    []stream `json:"streams"`
}

type stream struct {
	StreamName string    `json:"stream_name"`
	Start      time.Time `json:"start"`
	End        time.Time `json:"end"`
	Sources    []string  `json:"sources"`
	Live       bool      `json:"live"`
}
