package api

import (
	"github.com/gin-gonic/gin"

	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/pkg/runner_manager"
	"github.com/TUM-Dev/gocast/tools"
)

// ConfigGinRouter for non ws endpoints
func ConfigGinRouter(
	router *gin.Engine,
	manager *runner_manager.Manager,
) {
	daoWrapper := dao.NewDaoWrapper()

	router.Use(tools.ErrorHandler)

	configGinStreamRestRouter(router, daoWrapper, manager)
	configGinUsersRouter(router, daoWrapper)
	configGinCourseRouter(router, daoWrapper)
	configGinLectureHallApiRouter(router, daoWrapper)
	configTokenRouter(router, daoWrapper)
	configNotificationsRouter(router, daoWrapper)
	configGinSearchRouter(router, daoWrapper, tools.NewMeiliSearchFunctions())
	configSemestersRouter(router, daoWrapper)
	configSelfstreamRouter(router, daoWrapper, manager)
}
