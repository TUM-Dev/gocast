package api

import (
	"github.com/gin-gonic/gin"
	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/tools/realtime"
	"github.com/TUM-Dev/gocast/tools/realtime/connector"
	log "github.com/sirupsen/logrus"
)

type realtimeRoutes struct {
	dao.DaoWrapper
}

var RealtimeInstance = realtime.New(connector.NewMelodyConnector())

func configGinRealtimeRouter(router *gin.RouterGroup, daoWrapper dao.DaoWrapper) {
	routes := realtimeRoutes{daoWrapper}
	router.GET("/ws", routes.handleRealtimeConnect)
}

func (r realtimeRoutes) handleRealtimeConnect(c *gin.Context) {
	properties := make(map[string]interface{}, 1)
	properties["ctx"] = c
	properties["dao"] = r.DaoWrapper

	if err := RealtimeInstance.HandleRequest(c.Writer, c.Request, properties); err != nil {
		log.WithError(err).Warn("Something went wrong while handling Realtime-Socket request")
	}
}
