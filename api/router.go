package api

import (
	"github.com/gin-gonic/gin"
	"github.com/joschahenningsen/TUM-Live/dao"
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
	configGinLectureHallApiRouter(router, daoWrapper)
	configGinSexyApiRouter(router, daoWrapper)
	configProgressRouter(router, daoWrapper)
	configServerNotificationsRoutes(router, daoWrapper)
	configTokenRouter(router, daoWrapper)
	configWorkerRouter(router, daoWrapper)
	configNotificationsRouter(router, daoWrapper)
}
