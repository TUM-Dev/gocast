package api

import (
	"TUM-Live/tools"
	"github.com/gin-gonic/gin"
	"gopkg.in/olahol/melody.v1"
	"log"
)

var m *melody.Melody

func configGinChatRouter(router gin.IRoutes) {
	router.GET("/:vidId/ws", ChatStream)
	if m == nil {
		log.Printf("creating melody")
		m = melody.New()
	}
	m.HandleMessage(func(s *melody.Session, msg []byte) {
		ctx, found := s.Get("ctx") // get gin context
		if found {
			_, uErr := tools.GetUser(ctx.(*gin.Context))
			_, sErr := tools.GetStudent(ctx.(*gin.Context))
			if uErr != nil && sErr != nil {
				// not allowed to send message
				return
			}
		}
		// todo store message
		_ = m.BroadcastFilter(msg, func(q *melody.Session) bool { // filter broadcasting to same lecture.
			return q.Request.URL.Path == s.Request.URL.Path
		})
	})
}

func ChatStream(c *gin.Context) {
	ctxMap := make(map[string]interface{}, 1)
	ctxMap["ctx"] = c
	_ = m.HandleRequestWithKeys(c.Writer, c.Request, ctxMap)
}
