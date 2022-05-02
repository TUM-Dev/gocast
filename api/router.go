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
	configGinStreamRestRouter(router)
	configGinUsersRouter(router)
	configGinCourseRouter(router)
	configGinDownloadRouter(router, daoWrapper)
	configGinLectureHallApiRouter(router)
	configGinSexyApiRouter(router)
	configProgressRouter(router)
	configServerNotificationsRoutes(router)
	configTokenRouter(router)
	configWorkerRouter(router, daoWrapper)
	configNotificationsRouter(router)
}
