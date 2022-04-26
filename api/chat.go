package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/joschahenningsen/TUM-Live/dao"
	"github.com/joschahenningsen/TUM-Live/model"
	"github.com/joschahenningsen/TUM-Live/tools"
	"net/http"
	"strconv"
	"time"

	"github.com/gabstv/melody"
	"github.com/getsentry/sentry-go"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
)

var m *melody.Melody

const maxParticipants = 10000

func configGinChatRouter(router *gin.RouterGroup) {
	wsGroup := router.Group("/:streamID")

	wsGroup.Use(tools.InitStream)
	wsGroup.GET("/messages", getMessages)
	wsGroup.GET("/active-poll", getActivePoll)
	wsGroup.GET("/users", getUsers)
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
		case "start_poll":
			handleStartPoll(tumLiveContext, msg)
		case "submit_poll_option_vote":
			handleSubmitPollOptionVote(tumLiveContext, msg)
		case "close_active_poll":
			handleCloseActivePoll(tumLiveContext)
		case "resolve":
			handleResolve(tumLiveContext, msg)
		case "approve":
			handleApprove(tumLiveContext, msg)
		default:
			log.WithField("type", req.Type).Warn("unknown websocket request type")
		}
	})

	//delete closed sessions every second
	go func() {
		c := time.Tick(time.Second)
		for range c {
			cleanupSessions()
		}
	}()
}

func handleSubmitPollOptionVote(ctx tools.TUMLiveContext, msg []byte) {
	var req submitPollOptionVote
	if err := json.Unmarshal(msg, &req); err != nil {
		log.WithError(err).Warn("could not unmarshal submit poll answer request")
		return
	}
	if ctx.User == nil {
		return
	}

	if err := dao.Chat.AddChatPollOptionVote(req.PollOptionId, ctx.User.ID); err != nil {
		log.WithError(err).Warn("could not add poll option vote")
		return
	}

	voteCount, _ := dao.Chat.GetPollOptionVoteCount(req.PollOptionId)

	voteUpdateMap := gin.H{
		"pollOptionId": req.PollOptionId,
		"votes":        voteCount,
	}

	if voteUpdateJson, err := json.Marshal(voteUpdateMap); err == nil {
		broadcastStreamToAdmins(ctx.Stream.ID, voteUpdateJson)
	} else {
		log.WithError(err).Warn("could not marshal vote update map")
		return
	}
}

func handleStartPoll(ctx tools.TUMLiveContext, msg []byte) {
	type startPollReq struct {
		wsReq
		Question    string   `json:"question"`
		PollAnswers []string `json:"pollAnswers"`
	}

	var req startPollReq
	if err := json.Unmarshal(msg, &req); err != nil {
		log.WithError(err).Warn("could not unmarshal start poll request")
		return
	}
	if ctx.User == nil || !ctx.User.IsAdminOfCourse(*ctx.Course) {
		return
	}

	if len(req.Question) == 0 {
		log.Warn("could not create poll with empty question")
		return
	}

	var pollOptions []model.PollOption
	for _, answer := range req.PollAnswers {
		if len(answer) == 0 {
			log.Warn("could not create poll with empty answer")
			return
		}
		pollOptions = append(pollOptions, model.PollOption{
			Answer: answer,
		})
	}

	poll := model.Poll{
		StreamID:    ctx.Stream.ID,
		Question:    req.Question,
		Active:      true,
		PollOptions: pollOptions,
	}

	if err := dao.Chat.AddChatPoll(&poll); err != nil {
		return
	}

	var pollOptionsJson []gin.H
	for _, option := range poll.PollOptions {
		pollOptionsJson = append(pollOptionsJson, option.GetStatsMap(0))
	}

	pollMap := gin.H{
		"active":      true,
		"question":    poll.Question,
		"pollOptions": pollOptionsJson,
		"submitted":   0,
	}
	if pollJson, err := json.Marshal(pollMap); err == nil {
		broadcastStream(ctx.Stream.ID, pollJson)
	}
}

func handleCloseActivePoll(ctx tools.TUMLiveContext) {
	if ctx.User == nil || !ctx.User.IsAdminOfCourse(*ctx.Course) {
		return
	}

	poll, err := dao.Chat.GetActivePoll(ctx.Stream.ID)
	if err != nil {
		return
	}

	if err = dao.Chat.CloseActivePoll(ctx.Stream.ID); err != nil {
		return
	}

	var pollOptions []gin.H
	for _, option := range poll.PollOptions {
		voteCount, _ := dao.Chat.GetPollOptionVoteCount(option.ID)
		pollOptions = append(pollOptions, option.GetStatsMap(voteCount))
	}

	statsMap := gin.H{
		"question":          poll.Question,
		"pollOptionResults": pollOptions,
	}

	if statsJson, err := json.Marshal(statsMap); err == nil {
		broadcastStream(ctx.Stream.ID, statsJson)
	}
}

func handleResolve(ctx tools.TUMLiveContext, msg []byte) {
	var req resolveReq
	err := json.Unmarshal(msg, &req)
	if err != nil {
		log.WithError(err).Warn("could not unmarshal message delete request")
		return
	}
	if ctx.User == nil || !ctx.User.IsAdminOfCourse(*ctx.Course) {
		return
	}

	err = dao.Chat.ResolveChat(req.Id)
	if err != nil {
		log.WithError(err).Error("could not delete chat")
	}

	broadcast := gin.H{
		"resolve": req.Id,
	}
	broadcastBytes, err := json.Marshal(broadcast)
	if err != nil {
		log.WithError(err).Error("could not marshal delete message")
		return
	}
	broadcastStream(ctx.Stream.ID, broadcastBytes)
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
	err = dao.Chat.DeleteChat(req.Id)
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

func handleApprove(ctx tools.TUMLiveContext, msg []byte) {
	var req approveReq
	err := json.Unmarshal(msg, &req)
	if err != nil {
		log.WithError(err).Warn("could not unmarshal message approve request")
		return
	}
	if ctx.User == nil || !ctx.User.IsAdminOfCourse(*ctx.Course) {
		return
	}
	err = dao.Chat.ApproveChat(req.Id)
	if err != nil {
		log.WithError(err).Error("could not approve chat")
	}
	broadcast := gin.H{
		"approve": req.Id,
	}
	broadcastBytes, err := json.Marshal(broadcast)
	if err != nil {
		log.WithError(err).Error("could not marshal approve message")
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

	err = dao.Chat.ToggleLike(ctx.User.ID, req.Id)
	if err != nil {
		log.WithError(err).Error("error liking/unliking message")
		return
	}
	numLikes, err := dao.Chat.GetNumLikes(req.Id)
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

	isAdmin := ctx.User.IsAdminOfCourse(*ctx.Course)

	isVisible := sql.NullBool{Valid: true, Bool: true}
	if ctx.Course.ModeratedChatEnabled && !isAdmin {
		isVisible.Bool = false
	}
	chatForDb := model.Chat{
		UserID:         strconv.Itoa(int(ctx.User.ID)),
		UserName:       uname,
		Message:        chat.Msg,
		StreamID:       ctx.Stream.ID,
		Admin:          isAdmin,
		ReplyTo:        replyTo,
		Visible:        isVisible,
		IsVisible:      isVisible.Bool,
		AddressedToIds: chat.AddressedTo,
	}
	chatForDb.SanitiseMessage()
	err := dao.Chat.AddMessage(&chatForDb)
	if err != nil {
		if errors.Is(err, model.ErrCooledDown) {
			sendServerMessage("You are sending messages too fast. Please wait a bit.", TypeServerErr, session)
		}
		return
	}

	if msg, err := json.Marshal(chatForDb); err == nil {
		if ctx.Course.ModeratedChatEnabled && !isAdmin {
			_ = session.Write(msg)                      // send message back to sender
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

	isAdmin := false
	var uid uint = 0 // 0 = not logged in. -> doesn't match a user
	if tumLiveContext.User != nil {
		uid = tumLiveContext.User.ID
		isAdmin = tumLiveContext.User.IsAdminOfCourse(*tumLiveContext.Course)
	}

	var err error
	var chats []model.Chat
	if isAdmin {
		chats, err = dao.Chat.GetAllChats(uid, tumLiveContext.Stream.ID)
	} else {
		chats, err = dao.Chat.GetVisibleChats(uid, tumLiveContext.Stream.ID)
	}
	if err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	c.JSON(http.StatusOK, chats)
}

func getUsers(c *gin.Context) {
	foundContext, exists := c.Get("TUMLiveContext")
	if !exists {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	tumLiveContext := foundContext.(tools.TUMLiveContext)
	users, err := dao.Chat.GetChatUsers(tumLiveContext.Stream.ID)
	if err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	type chatUserSearchDto struct {
		ID   uint   `json:"id"`
		Name string `json:"name"`
	}
	resp := make([]chatUserSearchDto, len(users))
	for i, user := range users {
		resp[i].ID = user.ID
		resp[i].Name = user.Name
	}
	c.JSON(http.StatusOK, resp)
}

func getActivePoll(c *gin.Context) {
	foundContext, exists := c.Get("TUMLiveContext")
	if !exists {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}

	tumLiveContext := foundContext.(tools.TUMLiveContext)
	if tumLiveContext.User == nil {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	poll, err := dao.Chat.GetActivePoll(tumLiveContext.Stream.ID)
	if err != nil {
		c.JSON(http.StatusOK, nil)
		return
	}

	submitted, err := dao.Chat.GetPollUserVote(poll.ID, tumLiveContext.User.ID)
	if err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	isAdminOfCourse := tumLiveContext.User.IsAdminOfCourse(*tumLiveContext.Course)
	var pollOptions []gin.H
	for _, option := range poll.PollOptions {
		voteCount := int64(0)

		if isAdminOfCourse {
			voteCount, err = dao.Chat.GetPollOptionVoteCount(option.ID)
			if err != nil {
				log.WithError(err).Warn("could not get poll option vote count")
			}
		}

		pollOptions = append(pollOptions, option.GetStatsMap(voteCount))
	}

	c.JSON(http.StatusOK, gin.H{
		"active":      true,
		"question":    poll.Question,
		"pollOptions": pollOptions,
		"submitted":   submitted,
	})
}

type wsReq struct {
	Type string `json:"type"`
}

type chatReq struct {
	wsReq
	Msg         string `json:"msg"`
	Anonymous   bool   `json:"anonymous"`
	ReplyTo     int64  `json:"replyTo"`
	AddressedTo []uint `json:"addressedTo"`
}

type likeReq struct {
	wsReq
	Id uint `json:"id"`
}

type deleteReq struct {
	wsReq
	Id uint `json:"id"`
}

type submitPollOptionVote struct {
	wsReq
	PollOptionId uint `json:"pollOptionId"`
}

type resolveReq struct {
	wsReq
	Id uint `json:"id"`
}

type approveReq struct {
	wsReq
	Id uint `json:"id"`
}

func CollectStats() {
	BroadcastStats()
	for sID, sessions := range sessionsMap {
		stat := model.Stat{
			Time:     time.Now(),
			StreamID: sID,
			Viewers:  uint(len(sessions)),
			Live:     true,
		}
		if s, err := dao.Streams.GetStreamByID(context.Background(), fmt.Sprintf("%d", sID)); err == nil {
			if s.LiveNow { // store stats for livestreams only
				s.Stats = append(s.Stats, stat)
				if err := dao.Statistics.AddStat(stat); err != nil {
					log.WithError(err).Error("Saving stat failed")
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
		err := dao.Streams.AddVodView(id)
		if err != nil {
			log.WithError(err).Error("Can't save vod view")
		}
	}
}
