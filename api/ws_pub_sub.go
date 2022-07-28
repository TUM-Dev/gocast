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

var pubSubSocket *pubsub.PubSub

func configGinWSPubSubRouter(router *gin.RouterGroup, daoWrapper dao.DaoWrapper) {
	routes := wsPubSubRoutes{daoWrapper}

	if pubSubSocket == nil {
		log.Printf("creating pubsub socket")
		pubSubSocket = pubsub.New()
	}

	router.GET("/ws", routes.handleWSPubSubConnect)
}

func (r wsPubSubRoutes) handleWSPubSubConnect(c *gin.Context) {
	properties := make(map[string]interface{}, 1)
	properties["ctx"] = c
	properties["dao"] = r.DaoWrapper
	pubSubSocket.HandleRequest(c.Writer, c.Request, properties)
}
