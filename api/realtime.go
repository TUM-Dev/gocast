package api

import (
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/joschahenningsen/TUM-Live/dao"
	"github.com/mono424/go-pts"
	"github.com/mono424/go-pts-gorilla-connector"
	log "github.com/sirupsen/logrus"
)

type realtimeRoutes struct {
	dao.DaoWrapper
}

var PtsInstance = pts.New(ptsc_gorilla.NewConnector(
	websocket.Upgrader{},
	func(err *pts.Error) {
		log.Warn(err.Description)
	},
))

func configGinRealtimeRouter(router *gin.RouterGroup, daoWrapper dao.DaoWrapper) {
	routes := realtimeRoutes{daoWrapper}
	router.GET("/ws", routes.handleRealtimeConnect)
}

func (r realtimeRoutes) handleRealtimeConnect(c *gin.Context) {
	properties := make(map[string]interface{}, 1)
	properties["ctx"] = c
	properties["dao"] = r.DaoWrapper

	if err := PtsInstance.HandleRequest(c.Writer, c.Request, properties); err != nil {
		log.WithError(err).Warn("Something went wrong while handling Realtime-Socket request")
	}
}
