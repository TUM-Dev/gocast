package api

import (
	"errors"
	"github.com/gin-gonic/gin"
	"github.com/joschahenningsen/TUM-Live/dao"
	"github.com/joschahenningsen/TUM-Live/model"
	"github.com/joschahenningsen/TUM-Live/tools"
	"gorm.io/gorm"
	"net/http"
)

const (
	FILTER_FULL    = "full"
	FILTER_PARTIAL = "partial"
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

type coursesByIdQuery struct {
	FilterMode string `form:"filter,default=partial"`
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

	var query coursesByIdQuery
	if err := c.BindQuery(&query); err != nil {
		_ = c.Error(tools.RequestError{
			Err:           err,
			Status:        http.StatusBadRequest,
			CustomMessage: "invalid query",
		})
		return
	}

	type Response struct{ Course model.Course }

	course, err := r.CoursesDao.GetCourseById(c, uri.ID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			_ = c.Error(tools.RequestError{
				Status:        http.StatusBadRequest,
				CustomMessage: "can't find course",
			})
		} else {
			_ = c.Error(tools.RequestError{
				Err:           err,
				Status:        http.StatusInternalServerError,
				CustomMessage: "can't retrieve course",
			})
		}
		return
	}

	if query.FilterMode == FILTER_PARTIAL {
		course.Streams = nil
	}

	// Not-Logged-In Users do not receive the watch state
	if tumLiveContext.User == nil {
		c.JSON(http.StatusOK, Response{Course: course})
		return
	}
}
