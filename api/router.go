package api

import (
	"github.com/gin-gonic/gin"

	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/pkg/runner_manager"
	"github.com/TUM-Dev/gocast/tools"
)

// ConfigChatRouter configure gin router for chat (without gzip)
func ConfigChatRouter(router *gin.RouterGroup) {
	daoWrapper := dao.NewDaoWrapper()
	router.Use(tools.ErrorHandler)
	configGinChatRouter(router, daoWrapper)
}

// ConfigRealtimeRouter configure gin router for live-updates (without gzip)
func ConfigRealtimeRouter(router *gin.RouterGroup) {
	daoWrapper := dao.NewDaoWrapper()
	configGinRealtimeRouter(router, daoWrapper)

	// Register Channels
	RegisterLiveUpdateRealtimeChannel()
	RegisterLiveRunnerPageUpdateRealtimeChannel(daoWrapper)
	RegisterRealtimeChatChannel()
}

// ConfigGinRouter for non ws endpoints
func ConfigGinRouter(router *gin.Engine, manager *runner_manager.Manager) {
	daoWrapper := dao.NewDaoWrapper()

	router.Use(tools.ErrorHandler)

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
	configRunnerRouter(router, daoWrapper)
	configNotificationsRouter(router, daoWrapper)
	configInfoPageRouter(router, daoWrapper)
	configGinSearchRouter(router, daoWrapper, tools.NewMeiliSearchFunctions())
	configAuditRouter(router, daoWrapper)
	configGinBookmarksRouter(router, daoWrapper)
	configMaintenanceRouter(router, daoWrapper)
	configSemestersRouter(router, daoWrapper)
	configSelfstreamRouter(router, daoWrapper, manager)
}
