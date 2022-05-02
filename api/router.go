package api

import (
	"github.com/gin-gonic/gin"
	"github.com/joschahenningsen/TUM-Live/dao"
)

// ConfigChatRouter configure gin router for chat (without gzip)
func ConfigChatRouter(router *gin.RouterGroup) {
	configGinChatRouter(router)
}

//ConfigGinRouter for non ws endpoints
func ConfigGinRouter(router *gin.Engine) {
	daoWrapper := newDaoWrapper()
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

type DaoWrapper struct {
	dao.CameraPresetDao
	dao.ChatDao
	dao.FileDao
	dao.StreamsDao
	dao.CoursesDao
	dao.WorkerDao
}

func newDaoWrapper() DaoWrapper {
	return DaoWrapper{
		CameraPresetDao: dao.NewCameraPresetDao(),
		ChatDao:         dao.NewChatDao(),
		FileDao:         dao.NewFileDao(),
		StreamsDao:      dao.NewStreamsDao(),
		CoursesDao:      dao.NewCoursesDao(),
		WorkerDao:       dao.NewWorkerDao(),
	}
}
