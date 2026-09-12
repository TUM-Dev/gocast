package web

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/tools"
)

var VersionTag string

// IndexData is what every server-rendered page needs: who is asking, and how the
// installation presents itself. The course and semester listings it used to carry
// went with the start page, which the SPA serves.
type IndexData struct {
	VersionTag     string
	TUMLiveContext tools.TUMLiveContext
	CanonicalURL   tools.CanonicalURL
	Branding       tools.Branding
	WikiURL        string
	// Only the course editor still sets these, to mark a course as being of a past
	// semester. EditCoursePage fills them; nothing else does.
	CurrentYear int
	CurrentTerm string
}

func NewIndexData() IndexData {
	return IndexData{
		VersionTag:   VersionTag,
		CanonicalURL: tools.NewCanonicalURL(tools.Cfg.CanonicalURL),
		Branding:     tools.BrandingCfg,
		WikiURL:      tools.Cfg.WikiURL,
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
		logger.Warn("could not get TUMLiveContext")
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
