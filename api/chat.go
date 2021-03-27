package api

import (
	"TUM-Live/dao"
	"TUM-Live/tools"
	"github.com/gin-gonic/gin"
	"gopkg.in/olahol/melody.v1"
	"log"
	"net/http"
	"strconv"
	"sync"
)

var m *melody.Melody
var stats = map[string]int{}
var statsLock = sync.RWMutex{}

func configGinChatRouter(router gin.IRoutes) {
	router.GET("/:vidId/ws", ChatStream)
	router.GET("/:vidId/stats", ChatStats)
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

func ChatStats(context *gin.Context) {
	u, uErr := tools.GetUser(context)
	if uErr != nil || u.Role != 1 {
		context.AbortWithStatusJSON(http.StatusForbidden, gin.H{"msg": "stats can currently only be retrieved by admins"})
		return
	}
	vidId := context.Param("vidId")
	watchers, found := stats[vidId]
	if !found {
		context.AbortWithStatusJSON(http.StatusNotFound, gin.H{"msg": "stream not found"})
		return
	}
	context.JSON(http.StatusOK, gin.H{"watchers": watchers})
}

func ChatStream(c *gin.Context) {
	ctxMap := make(map[string]interface{}, 1)
	println(c.Param("vidId") + "joined")
	statsLock.Lock()
	stats[c.Param("vidId")] = stats[c.Param("vidId")] + 1
	statsLock.Unlock()
	m.HandleClose(func(session *melody.Session, i int, s string) error {
		statsLock.Lock()
		stats[c.Param("vidId")] = stats[c.Param("vidId")] - 1
		statsLock.Unlock()
		return nil
	})
	ctxMap["ctx"] = c
	_ = m.HandleRequestWithKeys(c.Writer, c.Request, ctxMap)
}
