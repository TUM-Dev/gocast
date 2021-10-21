package api

import (
	"TUM-Live/dao"
	"TUM-Live/model"
	"TUM-Live/tools"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gabstv/melody"
	"github.com/getsentry/sentry-go"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"net/http"
	"strconv"
	"sync"
	"time"
)

var m *melody.Melody
var stats = map[string]int{}
var statsLock = sync.RWMutex{}

func configGinChatRouter(router *gin.RouterGroup) {
	wsGroup := router.Group("/:streamID")
	wsGroup.Use(tools.InitStream)
	wsGroup.GET("/ws", ChatStream)
	if m == nil {
		log.Printf("creating melody")
		m = melody.New()
	}
	m.HandleConnect(connHandler)

	m.HandleMessage(func(s *melody.Session, msg []byte) {
		ctx, _ := s.Get("ctx") // get gin context
		foundContext, exists := ctx.(*gin.Context).Get("TUMLiveContext")
		if !exists {
			sentry.CaptureException(errors.New("context should exist but doesn't"))
			ctx.(*gin.Context).AbortWithStatus(http.StatusInternalServerError)
			return
		}
		tumLiveContext := foundContext.(tools.TUMLiveContext)
		if tumLiveContext.User == nil {
			return
		}
		var chat ChatReq
		if err := json.Unmarshal(msg, &chat); err != nil {
			return
		}
		if chat.Msg == "" || len(chat.Msg) > 200 {
			return
		}
		if dao.IsUserCooledDown(fmt.Sprintf("%v", tumLiveContext.User.ID)) {
			return
		}
		if !tumLiveContext.Course.ChatEnabled {
			return
		}
		uname := tumLiveContext.User.Name
		if chat.Anonymous {
			uname = "Anonymous"
		}
		dao.AddMessage(model.Chat{
			UserID:   strconv.Itoa(int(tumLiveContext.User.ID)),
			UserName: uname,
			Message:  chat.Msg,
			StreamID: tumLiveContext.Stream.ID,
			Admin:    tumLiveContext.User.ID == tumLiveContext.Course.UserID,
			SendTime: time.Now().In(tools.Loc),
		})
		if broadcast, err := json.Marshal(ChatRep{
			Msg:   chat.Msg,
			Name:  uname,
			Admin: tumLiveContext.User.ID == tumLiveContext.Course.UserID,
		}); err == nil {
			broadcastStream(tumLiveContext.Stream.ID, broadcast)
		}
	})
}

type ChatReq struct {
	Msg       string `json:"msg"`
	Anonymous bool   `json:"anonymous"`
}
type ChatRep struct {
	Msg   string `json:"msg"`
	Name  string `json:"name"`
	Admin bool   `json:"admin"`
}

func CollectStats() {
	BroadcastStats()
	for sID, sessions := range sessionsMap {
		stat := model.Stat{
			Time:    time.Now(),
			Viewers: len(sessions),
			Live:    true,
		}
		if s, err := dao.GetStreamByID(context.Background(), fmt.Sprintf("%d", sID)); err == nil {
			if s.LiveNow { // store stats for livestreams only
				s.Stats = append(s.Stats, stat)
				if err = dao.SaveStream(&s); err != nil {
					sentry.CaptureException(err)
					log.Printf("Error saving stats: %v\n", err)
				}
			}
		}
	}
}

func notifyViewersPause(streamId uint, paused bool) {
	req, _ := json.Marshal(gin.H{"paused": paused})
	broadcastStream(streamId, req)
}

func NotifyViewersLiveState(streamId uint, live bool) {
	req, _ := json.Marshal(gin.H{"live": live})
	broadcastStream(streamId, req)
}

func ChatStream(c *gin.Context) {
	// max participants in chat to prevent attacks
	if m.Len() > 10000 {
		return
	}
	joinTime := time.Now()
	foundContext, exists := c.Get("TUMLiveContext")
	if !exists {
		sentry.CaptureException(errors.New("context should exist but doesn't"))
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	tumLiveContext := foundContext.(tools.TUMLiveContext)
	defer afterDisconnect(c.Param("streamID"), joinTime, tumLiveContext.Stream.Recording)
	ctxMap := make(map[string]interface{}, 1)
	ctxMap["ctx"] = c

	_ = m.HandleRequestWithKeys(c.Writer, c.Request, ctxMap)
}

func afterDisconnect(id string, jointime time.Time, recording bool) {
	// watched at least 5 minutes of the lecture and stream is VoD? Count as view.
	if recording && jointime.Before(time.Now().Add(time.Minute*-5)) {
		err := dao.AddVodView(id)
		if err != nil {
			log.WithError(err).Error("Can't save vod view")
		}
	}
}
