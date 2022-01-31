package api

import (
	"TUM-Live/dao"
	"TUM-Live/model"
	"TUM-Live/tools"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gabstv/melody"
	"github.com/getsentry/sentry-go"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"net/http"
	"strconv"
	"time"
)

var m *melody.Melody

const maxParticipants = 10000

func configGinChatRouter(router *gin.RouterGroup) {
	wsGroup := router.Group("/:streamID")
	wsGroup.Use(tools.InitStream)
	wsGroup.GET("/messages", getMessages)
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
		var req wsReq
		err := json.Unmarshal(msg, &req)
		if err != nil {
			log.Warn(err)
			return
		}
		if req.Type == "message" {
			handleMessage(tumLiveContext, s, msg)
		} else if req.Type == "like" {
			handleLike(tumLiveContext, msg)
		}
	})
}

func handleLike(ctx tools.TUMLiveContext, msg []byte) {
	var req likeReq
	err := json.Unmarshal(msg, &req)
	if err != nil {
		return
	}
	err = dao.ToggleLike(ctx.User.ID, req.Id)
	if err != nil {
		log.WithError(err).Error("error liking/unliking message")
		return
	}
	numLikes, err := dao.GetNumLikes(req.Id)
	if err != nil {
		log.WithError(err).Error("error getting num of chat likes")
		return
	}
	broadcast := gin.H{
		"likes": req.Id,
		"num":   numLikes,
	}
	broadcastBytes, err := json.Marshal(broadcast)
	if err != nil {
		log.WithError(err).Warn("Can't marshal like message")
		return
	}
	broadcastStream(ctx.Stream.ID, broadcastBytes)
}

func handleMessage(ctx tools.TUMLiveContext, session *melody.Session, msg []byte) {
	var chat chatReq
	if err := json.Unmarshal(msg, &chat); err != nil {
		log.Info(err)
		return
	}

	if !ctx.Course.ChatEnabled {
		return
	}
	uname := ctx.User.Name
	if chat.Anonymous && ctx.Course.AnonymousChatEnabled {
		uname = "Anonymous"
	}
	replyTo := sql.NullInt64{}
	if chat.ReplyTo == 0 {
		replyTo.Int64 = 0
		replyTo.Valid = false
	} else {
		replyTo.Int64 = chat.ReplyTo
		replyTo.Valid = true
	}
	chatForDb := model.Chat{
		UserID:   strconv.Itoa(int(ctx.User.ID)),
		UserName: uname,
		Message:  chat.Msg,
		StreamID: ctx.Stream.ID,
		Admin:    ctx.User.ID == ctx.Course.UserID,
		ReplyTo:  replyTo,
	}
	err := dao.AddMessage(&chatForDb)
	if err != nil {
		if errors.Is(err, model.ErrCooledDown) {
			sendServerMessage("You are sending messages too fast. Please wait a bit.", TypeServerErr, session)
		}
		return
	}
	if broadcast, err := json.Marshal(chatForDb); err == nil {
		broadcastStream(ctx.Stream.ID, broadcast)
	}
}

func getMessages(c *gin.Context) {
	foundContext, exists := c.Get("TUMLiveContext")
	if !exists {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	tumLiveContext := foundContext.(tools.TUMLiveContext)
	chats, err := dao.GetChats(tumLiveContext.User.ID, tumLiveContext.Stream.ID)
	if err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	c.JSON(http.StatusOK, chats)
}

type wsReq struct {
	Type string `json:"type"`
}

type chatReq struct {
	wsReq
	Msg       string `json:"msg"`
	Anonymous bool   `json:"anonymous"`
	ReplyTo   int64  `json:"replyTo"`
}

type likeReq struct {
	wsReq
	Id uint `json:"id"`
}

type ChatRep struct {
	ID      uint   `json:"id"`
	Msg     string `json:"msg"`
	Name    string `json:"name"`
	Admin   bool   `json:"admin"`
	ReplyTo uint   `json:"replyTo"`
}

func CollectStats() {
	BroadcastStats()
	for sID, sessions := range sessionsMap {
		stat := model.Stat{
			Time:    time.Now(),
			Viewers: uint(len(sessions)),
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
	if m.Len() > maxParticipants {
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
