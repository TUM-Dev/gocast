package api

import (
	"TUM-Live/dao"
	"TUM-Live/tools"
	"github.com/gin-gonic/gin"
	"gopkg.in/olahol/melody.v1"
	"log"
	"strconv"
)

var m *melody.Melody

func configGinChatRouter(router gin.IRoutes) {
	router.GET("/:vidId/ws", ChatStream)
	if m == nil {
		log.Printf("creating melody")
		m = melody.New()
	}
	m.HandleMessage(func(s *melody.Session, msg []byte) {
		ctx, _ := s.Get("ctx") // get gin context

		user, uErr := tools.GetUser(ctx.(*gin.Context))
		student, sErr := tools.GetStudent(ctx.(*gin.Context))
		if uErr != nil && sErr != nil {
			// not allowed to send message
			return

		}
		var uid string
		if uErr == nil {
			uid = strconv.Itoa(int(user.ID))
		} else if sErr == nil {
			uid = student.ID
		}
		vID, err := strconv.Atoi(ctx.(*gin.Context).Param("vidId"))
		if err != nil {
			return
		}
		dao.AddMessage(string(msg), uid, uint(vID))
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
