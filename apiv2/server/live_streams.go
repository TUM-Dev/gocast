package apiv2

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

type liveStreamDto struct {
	ID          uint
	CourseName  string
	LectureHall string
	COMB        string
	PRES        string
	CAM         string
	End         time.Time
}

// handleLiveStreams returns all streams that are live, for API users with an admin
// token (scripts, monitoring). The token check is middleware in front of it, see
// RegisterLegacyRoutes.
func (a *API) handleLiveStreams(c *gin.Context) {
	var res []liveStreamDto
	streams, err := a.dao.StreamsDao.GetCurrentLive(c)
	if err != nil {
		a.log.Error("Error getting current live", "err", err)
		abortLegacyError(c, http.StatusInternalServerError, "can not get current live streams", err)
		return
	}
	for _, s := range streams {
		course, err := a.dao.CoursesDao.GetCourseById(c, s.CourseID)
		if err != nil {
			a.log.Error("Error fetching course", "err", err)
		}
		lectureHall := "Selfstream"
		if s.LectureHallID != 0 {
			l, err := a.dao.LectureHallsDao.GetLectureHallByID(s.LectureHallID)
			if err != nil {
				a.log.Error("Error fetching lecture hall", "err", err)
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
