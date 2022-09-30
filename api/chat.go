package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/joschahenningsen/TUM-Live/tools/stats"
	"net/http"
	"strconv"
	"time"

	"gorm.io/gorm"

	"github.com/joschahenningsen/TUM-Live/dao"
	"github.com/joschahenningsen/TUM-Live/model"
	"github.com/joschahenningsen/TUM-Live/tools"
	"github.com/joschahenningsen/TUM-Live/tools/realtime"

	"github.com/getsentry/sentry-go"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
)

const (
	ChatRoomName = "chat/:streamID"
)

var routes chatRoutes

func RegisterRealtimeChatChannel() {
	RealtimeInstance.RegisterChannel(ChatRoomName, realtime.ChannelHandlers{
		SubscriptionMiddlewares: []realtime.SubscriptionMiddleware{
			tools.InitStreamRealtime(),
		},
		OnSubscribe:   chatOnSubscribe,
		OnUnsubscribe: chatOnUnsubscribe,
		OnMessage: func(psc *realtime.Context, message *realtime.Message) {
			foundContext, exists := psc.Get("TUMLiveContext")
			if !exists {
				sentry.CaptureException(errors.New("context should exist but doesn't"))
				return
			}
			tumLiveContext := foundContext.(tools.TUMLiveContext)
			if tumLiveContext.User == nil {
				return
			}

			req, err := parseChatPayload(message)

			if err != nil {
				log.WithError(err).Warn("could not unmarshal request")
				return
			}

			switch req.Type {
			case "message":
				routes.handleMessage(tumLiveContext, psc, message.Payload)
			case "like":
				routes.handleLike(tumLiveContext, message.Payload)
			case "delete":
				routes.handleDelete(tumLiveContext, message.Payload)
			case "start_poll":
				routes.handleStartPoll(tumLiveContext, message.Payload)
			case "submit_poll_option_vote":
				routes.handleSubmitPollOptionVote(tumLiveContext, message.Payload)
			case "close_active_poll":
				routes.handleCloseActivePoll(tumLiveContext)
			case "resolve":
				routes.handleResolve(tumLiveContext, message.Payload)
			case "approve":
				routes.handleApprove(tumLiveContext, message.Payload)
			default:
				log.WithField("type", req.Type).Warn("unknown websocket request type")
			}
		},
	})

	//delete closed sessions every second
	go func() {
		c := time.Tick(time.Second)
		for range c {
			cleanupSessions()
		}
	}()
}

func configGinChatRouter(router *gin.RouterGroup, daoWrapper dao.DaoWrapper) {
	routes = chatRoutes{daoWrapper}

	wsGroup := router.Group("/:streamID")
	wsGroup.Use(tools.InitStream(daoWrapper))
	wsGroup.GET("/messages", routes.getMessages)
	wsGroup.GET("/active-poll", routes.getActivePoll)
	wsGroup.GET("/users", routes.getUsers)
}

type chatRoutes struct {
	dao.DaoWrapper
}

func (r chatRoutes) handleSubmitPollOptionVote(ctx tools.TUMLiveContext, msg []byte) {
	var req submitPollOptionVote
	if err := json.Unmarshal(msg, &req); err != nil {
		log.WithError(err).Warn("could not unmarshal submit poll answer request")
		return
	}
	if ctx.User == nil {
		return
	}

	if err := r.ChatDao.AddChatPollOptionVote(req.PollOptionId, ctx.User.ID); err != nil {
		log.WithError(err).Warn("could not add poll option vote")
		return
	}

	voteCount, _ := r.ChatDao.GetPollOptionVoteCount(req.PollOptionId)

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

func (r chatRoutes) handleStartPoll(ctx tools.TUMLiveContext, msg []byte) {
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

	if err := r.ChatDao.AddChatPoll(&poll); err != nil {
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

func (r chatRoutes) handleCloseActivePoll(ctx tools.TUMLiveContext) {
	if ctx.User == nil || !ctx.User.IsAdminOfCourse(*ctx.Course) {
		return
	}

	poll, err := r.ChatDao.GetActivePoll(ctx.Stream.ID)
	if err != nil {
		return
	}

	if err = r.ChatDao.CloseActivePoll(ctx.Stream.ID); err != nil {
		return
	}

	var pollOptions []gin.H
	for _, option := range poll.PollOptions {
		voteCount, _ := r.ChatDao.GetPollOptionVoteCount(option.ID)
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

func (r chatRoutes) handleResolve(ctx tools.TUMLiveContext, msg []byte) {
	var req resolveReq
	err := json.Unmarshal(msg, &req)
	if err != nil {
		log.WithError(err).Warn("could not unmarshal message delete request")
		return
	}
	if ctx.User == nil || !ctx.User.IsAdminOfCourse(*ctx.Course) {
		return
	}

	err = r.ChatDao.ResolveChat(req.Id)
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

func (r chatRoutes) handleDelete(ctx tools.TUMLiveContext, msg []byte) {
	var req deleteReq
	err := json.Unmarshal(msg, &req)
	if err != nil {
		log.WithError(err).Warn("could not unmarshal message delete request")
		return
	}
	if ctx.User == nil || !ctx.User.IsAdminOfCourse(*ctx.Course) {
		return
	}
	err = r.ChatDao.DeleteChat(req.Id)
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

func (r chatRoutes) handleApprove(ctx tools.TUMLiveContext, msg []byte) {
	var req approveReq
	err := json.Unmarshal(msg, &req)
	if err != nil {
		log.WithError(err).Warn("could not unmarshal message approve request")
		return
	}
	if ctx.User == nil || !ctx.User.IsAdminOfCourse(*ctx.Course) {
		return
	}
	err = r.ChatDao.ApproveChat(req.Id)
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

func (r chatRoutes) handleLike(ctx tools.TUMLiveContext, msg []byte) {
	var req likeReq
	err := json.Unmarshal(msg, &req)
	if err != nil {
		log.WithError(err).Warn("could not unmarshal like request")
		return
	}

	err = r.ChatDao.ToggleLike(ctx.User.ID, req.Id)
	if err != nil {
		log.WithError(err).Error("error liking/unliking message")
		return
	}
	numLikes, err := r.ChatDao.GetNumLikes(req.Id)
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

func (r chatRoutes) handleMessage(ctx tools.TUMLiveContext, context *realtime.Context, msg []byte) {
	var chat chatReq
	if err := json.Unmarshal(msg, &chat); err != nil {
		log.WithError(err).Error("error unmarshaling chat message")
		return
	}
	if !ctx.Course.ChatEnabled {
		return
	}
	uname := ctx.User.GetPreferredName()
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
	err := r.ChatDao.AddMessage(&chatForDb)
	if err != nil {
		if errors.Is(err, model.ErrCooledDown) {
			sendServerMessage("You are sending messages too fast. Please wait a bit.", TypeServerErr, context)
		}
		return
	}

	if msg, err := json.Marshal(chatForDb); err == nil {
		if ctx.Course.ModeratedChatEnabled && !isAdmin {
			_ = context.Send(msg)                       // send message back to sender
			broadcastStreamToAdmins(ctx.Stream.ID, msg) // send message to course admins
		} else {
			broadcastStream(ctx.Stream.ID, msg)
		}
	}
}

func (r chatRoutes) getMessages(c *gin.Context) {
	foundContext, exists := c.Get("TUMLiveContext")
	if !exists {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusNotFound,
			CustomMessage: "context should exist but doesn't",
		})
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
		chats, err = r.ChatDao.GetAllChats(uid, tumLiveContext.Stream.ID)
	} else {
		chats, err = r.ChatDao.GetVisibleChats(uid, tumLiveContext.Stream.ID)
	}
	if err != nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusInternalServerError,
			CustomMessage: "can not get chat messages",
			Err:           err,
		})
		return
	}
	c.JSON(http.StatusOK, chats)
}

func (r chatRoutes) getUsers(c *gin.Context) {
	foundContext, exists := c.Get("TUMLiveContext")
	if !exists {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusNotFound,
			CustomMessage: "context should exist but doesn't",
		})
		return
	}
	tumLiveContext := foundContext.(tools.TUMLiveContext)
	users, err := r.ChatDao.GetChatUsers(tumLiveContext.Stream.ID)
	if err != nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusInternalServerError,
			CustomMessage: "can not get chat users",
			Err:           err,
		})
		return
	}
	type chatUserSearchDto struct {
		ID   uint   `json:"id"`
		Name string `json:"name"`
	}
	resp := make([]chatUserSearchDto, len(users))
	for i, user := range users {
		resp[i].ID = user.ID
		resp[i].Name = user.GetPreferredName()
	}
	c.JSON(http.StatusOK, resp)
}

func (r chatRoutes) getActivePoll(c *gin.Context) {
	foundContext, exists := c.Get("TUMLiveContext")
	if !exists {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusInternalServerError,
			CustomMessage: "context should exist but doesn't",
		})
		return
	}

	tumLiveContext := foundContext.(tools.TUMLiveContext)
	if tumLiveContext.User == nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusOK,
			CustomMessage: "not logged in",
		})
		return
	}
	poll, err := r.ChatDao.GetActivePoll(tumLiveContext.Stream.ID)
	if err != nil && err == gorm.ErrRecordNotFound {
		c.JSON(http.StatusOK, nil)
		return
	}
	if err != nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusInternalServerError,
			CustomMessage: "Can't get active poll",
			Err:           err,
		})
		return
	}

	submitted, err := r.ChatDao.GetPollUserVote(poll.ID, tumLiveContext.User.ID)
	if err != nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusInternalServerError,
			CustomMessage: "can not get poll user vote",
			Err:           err,
		})
		return
	}

	isAdminOfCourse := tumLiveContext.User.IsAdminOfCourse(*tumLiveContext.Course)
	var pollOptions []gin.H
	for _, option := range poll.PollOptions {
		voteCount := int64(0)

		if isAdminOfCourse {
			voteCount, err = r.ChatDao.GetPollOptionVoteCount(option.ID)
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

func parseChatPayload(m *realtime.Message) (res wsReq, err error) {
	dbByte, _ := json.Marshal(m.Payload)
	err = json.Unmarshal(dbByte, &res)
	return res, err
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

func CollectStats(daoWrapper dao.DaoWrapper) func() {
	return func() {
		BroadcastStats(daoWrapper.StreamsDao)
		for sID, sessions := range sessionsMap {
			if len(sessions) == 0 {
				continue
			}
			stat := model.Stat{
				Time:     time.Now(),
				StreamID: sID,
				Viewers:  uint(len(sessions)),
				Live:     true,
			}
			if s, err := daoWrapper.GetStreamByID(context.Background(), fmt.Sprintf("%d", sID)); err == nil {
				if s.LiveNow { // store stats for livestreams only
					if err := stats.Client.AddStreamStat(fmt.Sprintf("%d", s.CourseID), stat); err != nil {
						log.WithError(err).Error("Saving stat failed")
					}
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

func chatOnSubscribe(psc *realtime.Context) {
	joinTime := time.Now()
	psc.Set("chat.joinTime", joinTime)

	connHandler(psc)
}

func chatOnUnsubscribe(psc *realtime.Context) {
	var daoWrapper dao.DaoWrapper
	if ctx, ok := psc.Client.Get("dao"); ok {
		daoWrapper = ctx.(dao.DaoWrapper)
	} else {
		sentry.CaptureException(errors.New("daoWrapper should exist but doesn't"))
		return
	}

	var tumLiveContext tools.TUMLiveContext
	if foundContext, exists := psc.Get("TUMLiveContext"); exists {
		tumLiveContext = foundContext.(tools.TUMLiveContext)
	} else {
		sentry.CaptureException(errors.New("tumLiveContext should exist but doesn't"))
		return
	}

	var joinTime time.Time
	if result, exists := psc.Get("chat.joinTime"); exists {
		joinTime = result.(time.Time)
	} else {
		sentry.CaptureException(errors.New("joinTime should exist but doesn't"))
		return
	}

	defer afterUnsubscribe(psc.Param("streamID"), joinTime, tumLiveContext.Stream.Recording, daoWrapper)
}

func afterUnsubscribe(id string, joinTime time.Time, recording bool, daoWrapper dao.DaoWrapper) {
	// watched at least 5 minutes of the lecture and stream is VoD? Count as view.
	if recording && joinTime.Before(time.Now().Add(time.Minute*-5)) {
		err := daoWrapper.AddVodView(id)
		if err != nil {
			log.WithError(err).Error("Can't save vod view")
		}
	}
}
