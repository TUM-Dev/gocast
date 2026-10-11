package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/tools"
)

func configGinStreamRestRouter(router *gin.Engine, daoWrapper dao.DaoWrapper) {
	routes := streamRoutes{daoWrapper}

	// Endpoint for API users with token
	router.GET("/api/stream/live", tools.AdminToken(daoWrapper), routes.liveStreams)
}

type streamRoutes struct {
	dao.DaoWrapper
}

type liveStreamDto struct {
	ID          uint
	CourseName  string
	LectureHall string
	COMB        string
	PRES        string
	CAM         string
	End         time.Time
}

// livestreams returns all streams that are live
func (r streamRoutes) liveStreams(c *gin.Context) {
	var res []liveStreamDto
	streams, err := r.StreamsDao.GetCurrentLive(c)
	if err != nil {
		logger.Error("Error getting current live", "err", err)
		_ = c.Error(tools.RequestError{
			Status:        http.StatusInternalServerError,
			CustomMessage: "can not get current live streams",
			Err:           err,
		})
		return
	}
	for _, s := range streams {
		course, err := r.CoursesDao.GetCourseById(c, s.CourseID)
		if err != nil {
			logger.Error("Error fetching course", "err", err)
		}
		lectureHall := "Selfstream"
		if s.LectureHallID != 0 {
			l, err := r.LectureHallsDao.GetLectureHallByID(s.LectureHallID)
			if err != nil {
				logger.Error("Error fetching lecture hall", "err", err)
			} else {
				lectureHall = l.Name
			}
		}
		res = append(res, liveStreamDto{
			ID:          s.ID,
			CourseName:  course.Name,
			LectureHall: lectureHall,
			COMB:        s.PlaylistUrl,
			PRES:        s.PlaylistUrlPRES,
			CAM:         s.PlaylistUrlCAM,
			End:         s.End,
		})
	}
	c.JSON(http.StatusOK, res)
}
