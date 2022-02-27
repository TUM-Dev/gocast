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
			log.WithError(err).Warn("could not unmarshal request")
			return
		}
		switch req.Type {
		case "message":
			handleMessage(tumLiveContext, s, msg)
		case "like":
			handleLike(tumLiveContext, msg)
		case "delete":
			handleDelete(tumLiveContext, msg)
		default:
			log.WithField("type", req.Type).Warn("unknown websocket request type")
		}
	})
}

func handleDelete(ctx tools.TUMLiveContext, msg []byte) {
	var req deleteReq
	err := json.Unmarshal(msg, &req)
	if err != nil {
		log.WithError(err).Warn("could not unmarshal message delete request")
		return
	}
	if ctx.User == nil || !ctx.User.IsAdminOfCourse(*ctx.Course) {
		return
	}
	err = dao.DeleteChat(req.Id)
	if err != nil {
		log.WithError(err).Error("could not delete chat")
	}
	broadcast := gin.H{
		"delete": req.Id,
	}
	broadcastBytes, err := json.Marshal(broadcast)
	if err != nil {
		log.WithError(err).Error("could not marshal delete message")
		return
	}
	broadcastStream(ctx.Stream.ID, broadcastBytes)
}

func handleLike(ctx tools.TUMLiveContext, msg []byte) {
	var req likeReq
	err := json.Unmarshal(msg, &req)
	if err != nil {
		log.WithError(err).Warn("could not unmarshal like request")
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
		log.WithError(err).Error("Can't marshal like message")
		return
	}
	broadcastStream(ctx.Stream.ID, broadcastBytes)
}

func handleMessage(ctx tools.TUMLiveContext, session *melody.Session, msg []byte) {
	var chat chatReq
	if err := json.Unmarshal(msg, &chat); err != nil {
		log.WithError(err).Error("error unmarshaling chat message")
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
	isAdmin := ctx.User.ID == ctx.Course.UserID
	nb := sql.NullBool{Valid: true, Bool: true}
	if ctx.Course.ModeratedChatEnabled && !isAdmin {
		nb.Bool = false
	}

	chatForDb := model.Chat{
		UserID:   strconv.Itoa(int(ctx.User.ID)),
		UserName: uname,
		Message:  chat.Msg,
		StreamID: ctx.Stream.ID,
		Admin:    isAdmin,
		ReplyTo:  replyTo,
		Visible:  nb,
	}
	chatForDb.SanitiseMessage()
	err := dao.AddMessage(&chatForDb)
	if err != nil {
		if errors.Is(err, model.ErrCooledDown) {
			sendServerMessage("You are sending messages too fast. Please wait a bit.", TypeServerErr, session)
		}
		return
	}

	if msg, err := json.Marshal(chatForDb); err == nil {
		if ctx.Course.ModeratedChatEnabled && !isAdmin {
			_ = session.Write(msg) // send message back to sender
			broadcastStreamToAdmins(ctx.Stream.ID, msg) // send message to course admins
		} else {
			broadcastStream(ctx.Stream.ID, msg)
		}
	}
}

func getMessages(c *gin.Context) {
	foundContext, exists := c.Get("TUMLiveContext")
	if !exists {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	tumLiveContext := foundContext.(tools.TUMLiveContext)
	var uid uint = 0 // 0 = not logged in. -> doesn't match a user
	if tumLiveContext.User != nil {
		uid = tumLiveContext.User.ID
	}
	var err error
	var chats []model.Chat
	if tumLiveContext.User.IsAdminOfCourse(*tumLiveContext.Course) {
		chats, err = dao.GetAllChats(uid, tumLiveContext.Stream.ID)
	} else {
		chats, err = dao.GetVisibleChats(uid, tumLiveContext.Stream.ID)
	}
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

type deleteReq struct {
	wsReq
	Id uint `json:"id"`
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
