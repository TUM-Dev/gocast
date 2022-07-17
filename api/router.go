package api

import (
	"github.com/gin-gonic/gin"
	"github.com/joschahenningsen/TUM-Live/dao"
	"github.com/joschahenningsen/TUM-Live/tools"
)

// ConfigChatRouter configure gin router for chat (without gzip)
func ConfigChatRouter(router *gin.RouterGroup) {
	daoWrapper := dao.NewDaoWrapper()
	configGinChatRouter(router, daoWrapper)
}

// ConfigLiveUpdateRouter configure gin router for live-updates (without gzip)
func ConfigLiveUpdateRouter(router *gin.RouterGroup) {
	daoWrapper := dao.NewDaoWrapper()
	configGinLiveUpdateRouter(router, daoWrapper)
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
	configProgressRouter(router, daoWrapper)
	configSeekStatsRouter(router, daoWrapper)
	configServerNotificationsRoutes(router, daoWrapper)
	configTokenRouter(router, daoWrapper)
	configWorkerRouter(router, daoWrapper)
	configNotificationsRouter(router, daoWrapper)
	configInfoPageRouter(router, daoWrapper)
	configGinSearchRouter(router, daoWrapper)
	configAuditRouter(router, daoWrapper)
}
