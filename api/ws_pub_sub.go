package api

import (
	"github.com/gin-gonic/gin"
	"github.com/joschahenningsen/TUM-Live/dao"
	"github.com/joschahenningsen/TUM-Live/tools/pubsub"
	log "github.com/sirupsen/logrus"
)

type wsPubSubRoutes struct {
	dao.DaoWrapper
}

var PubSubSocket = pubsub.New()

func configGinWSPubSubRouter(router *gin.RouterGroup, daoWrapper dao.DaoWrapper) {
	routes := wsPubSubRoutes{daoWrapper}
	router.GET("/ws", routes.handleWSPubSubConnect)
}

func (r wsPubSubRoutes) handleWSPubSubConnect(c *gin.Context) {
	properties := make(map[string]interface{}, 1)
	properties["ctx"] = c
	properties["dao"] = r.DaoWrapper

	if err := PubSubSocket.HandleRequest(c.Writer, c.Request, properties); err != nil {
		log.WithError(err).Warn("Something went wrong while handling PubSub-Socket request")
	}
}
