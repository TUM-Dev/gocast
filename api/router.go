package api

import (
	"github.com/gin-gonic/gin"
	"github.com/joschahenningsen/TUM-Live/dao"
	"github.com/joschahenningsen/TUM-Live/tools"
	log "github.com/sirupsen/logrus"
	"net/http"
)

// ConfigChatRouter configure gin router for chat (without gzip)
func ConfigChatRouter(router *gin.RouterGroup) {
	daoWrapper := dao.NewDaoWrapper()
	configGinChatRouter(router, daoWrapper)
}

//ConfigGinRouter for non ws endpoints
func ConfigGinRouter(router *gin.Engine) {
	daoWrapper := dao.NewDaoWrapper()

	configGinStreamRestRouter(router, daoWrapper)
	configGinUsersRouter(router, daoWrapper)
	configGinCourseRouter(router, daoWrapper)
	configGinDownloadRouter(router, daoWrapper)
	configGinDownloadICSRouter(router, daoWrapper)
	configGinLectureHallApiRouter(router, daoWrapper, tools.NewPresetUtility(daoWrapper.LectureHallsDao))
	configGinSexyApiRouter(router, daoWrapper)
	configProgressRouter(router, daoWrapper)
	configServerNotificationsRoutes(router, daoWrapper)
	configTokenRouter(router, daoWrapper)
	configWorkerRouter(router, daoWrapper)
	configNotificationsRouter(router, daoWrapper)
	configInfoPageRouter(router, daoWrapper)
	configGinSearchRouter(router, daoWrapper)
	configAuditRouter(router, daoWrapper)
}

type Error struct {
	Status  int
	Error   error
	Message string
}

func HandleError(c *gin.Context, err Error) {
	if err.Status == http.StatusInternalServerError {
		log.WithError(err.Error).Error(err.Message)
	}
	j := gin.H{
		"status":  err.Status,
		"message": err.Message,
	}
	if err.Error != nil {
		j["error"] = err.Error.Error()
	}
	c.JSON(err.Status, j)
}
