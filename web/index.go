package web

import (
	"context"
	"github.com/gin-gonic/gin"
	"github.com/joschahenningsen/TUM-Live/dao"
	"github.com/joschahenningsen/TUM-Live/model"
	"github.com/joschahenningsen/TUM-Live/tools"
	log "github.com/sirupsen/logrus"
	"html/template"
	"net/http"
)

var VersionTag string

func (r mainRoutes) InfoPage(id uint) gin.HandlerFunc {
	return func(c *gin.Context) {
		var indexData IndexData
		var tumLiveContext tools.TUMLiveContext
		tumLiveContextQueried, found := c.Get("TUMLiveContext")
		if found {
			tumLiveContext = tumLiveContextQueried.(tools.TUMLiveContext)
			indexData.TUMLiveContext = tumLiveContext
		} else {
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}

		indexData = NewIndexData()

		text, err := r.InfoPageDao.GetById(id)
		if err != nil {
			log.WithError(err).Error("Could not get text with id")
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
		_ = templateExecutor.ExecuteTemplate(c.Writer, "info-page.gohtml", struct {
			IndexData
			Text template.HTML
		}{indexData, text.Render()})
	}
}

type IndexData struct {
	VersionTag          string
	TUMLiveContext      tools.TUMLiveContext
	IsUser              bool
	IsAdmin             bool
	IsStudent           bool
	LiveStreams         []CourseStream
	Courses             []model.Course
	PinnedCourses       []model.Course
	PublicCourses       []model.Course
	Semesters           []dao.Semester
	CurrentYear         int
	CurrentTerm         string
	UserName            string
	ServerNotifications []model.ServerNotification
	Branding            tools.Branding
}

func NewIndexData() IndexData {
	return IndexData{
		VersionTag: VersionTag,
		Branding:   tools.BrandingCfg,
	}
}

func NewIndexDataWithContext(c *gin.Context) IndexData {
	indexData := NewIndexData()

	var tumLiveContext tools.TUMLiveContext
	tumLiveContextQueried, found := c.Get("TUMLiveContext")
	if found {
		tumLiveContext = tumLiveContextQueried.(tools.TUMLiveContext)
		indexData.TUMLiveContext = tumLiveContext
	} else {
		log.Warn("could not get TUMLiveContext")
		c.AbortWithStatus(http.StatusInternalServerError)
	}

	return indexData
}

// IsFreshInstallation Checks whether there are users in the database and
// returns true if so, false if not.
func IsFreshInstallation(c *gin.Context, usersDao dao.UsersDao) (bool, error) {
	res, err := usersDao.AreUsersEmpty(context.Background()) // fresh installation?
	if err != nil {
		return false, err
	} else if res {
		return true, nil
	}

	return false, nil
}

type CourseStream struct {
	Course      model.Course
	Stream      model.Stream
	LectureHall *model.LectureHall
}
