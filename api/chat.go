package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"gorm.io/gorm"

	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/model"
	"github.com/TUM-Dev/gocast/tools"
	"github.com/TUM-Dev/gocast/tools/realtime"

	"github.com/getsentry/sentry-go"
	"github.com/gin-gonic/gin"
)

const (
	ChatRoomName = "chat/:streamID"
)

var allowedReactions = map[string]struct{}{
	"+1":       {},
	"-1":       {},
	"smile":    {},
	"tada":     {},
	"confused": {},
	"heart":    {},
	"eyes":     {},
}

const (
	POLL_START_MSG         = "start_poll"
	POLL_CLOSE_MSG         = "close_active_poll"
	POLL_PARTICIPATION_MSG = "submit_poll_option_vote"
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
				logger.Warn("could not unmarshal request", "err", err)
				return
			}

			switch req.Type {
			case "message":
				routes.handleMessage(tumLiveContext, psc, message.Payload)
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
			case "retract":
				routes.handleRetract(tumLiveContext, message.Payload)
			case "react_to":
				routes.handleReactTo(tumLiveContext, message.Payload)
			default:
				logger.Warn("unknown websocket request type", "type", req.Type)
			}
		},
	})

	// delete closed sessions every second
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
	wsGroup.GET("/polls", routes.getPolls)
}

type chatRoutes struct {
	dao.DaoWrapper
}

func (r chatRoutes) handleSubmitPollOptionVote(ctx tools.TUMLiveContext, msg []byte) {
	var req submitPollOptionVote
	if err := json.Unmarshal(msg, &req); err != nil {
		logger.Warn("could not unmarshal submit poll answer request", "err", err)
		return
	}
	if ctx.User == nil {
		return
	}

	if err := r.ChatDao.AddChatPollOptionVote(req.PollOptionId, ctx.User.ID); err != nil {
		logger.Warn("could not add poll option vote", "err", err)
		return
	}

	voteCount, _ := r.ChatDao.GetPollOptionVoteCount(req.PollOptionId)

	voteUpdateMap := gin.H{
		"type":         POLL_PARTICIPATION_MSG,
		"pollOptionId": req.PollOptionId,
		"votes":        voteCount,
	}

	if voteUpdateJson, err := json.Marshal(voteUpdateMap); err == nil {
		broadcastStreamToAdmins(ctx.Stream.ID, voteUpdateJson)
	} else {
		logger.Warn("could not marshal vote update map", "err", err)
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
		logger.Warn("could not unmarshal start poll request", "err", err)
		return
	}
	if ctx.User == nil || !ctx.User.IsAdminOfCourse(*ctx.Course) {
		return
	}

	if len(req.Question) == 0 {
		logger.Warn("could not create poll with empty question")
		return
	}

	var pollOptions []model.PollOption
	for _, answer := range req.PollAnswers {
		if len(answer) == 0 {
			logger.Warn("could not create poll with empty answer")
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
		"type":      POLL_START_MSG,
		"active":    true,
		"question":  poll.Question,
		"options":   pollOptionsJson,
		"submitted": 0,
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
		"type":     POLL_CLOSE_MSG,
		"question": poll.Question,
		"options":  pollOptions,
	}

	if statsJson, err := json.Marshal(statsMap); err == nil {
		broadcastStream(ctx.Stream.ID, statsJson)
	}
}

func (r chatRoutes) handleResolve(ctx tools.TUMLiveContext, msg []byte) {
	var req wsIdReq
	err := json.Unmarshal(msg, &req)
	if err != nil {
		logger.Warn("could not unmarshal message delete request", "err", err)
		return
	}
	if ctx.User == nil || !ctx.User.IsAdminOfCourse(*ctx.Course) {
		return
	}

	err = r.ChatDao.ResolveChat(req.Id)
	if err != nil {
		logger.Error("could not delete chat", "err", err)
	}

	broadcast := gin.H{
		"resolve": req.Id,
	}
	broadcastBytes, err := json.Marshal(broadcast)
	if err != nil {
		logger.Error("could not marshal delete message", "err", err)
		return
	}
	broadcastStream(ctx.Stream.ID, broadcastBytes)
}

func (r chatRoutes) handleDelete(ctx tools.TUMLiveContext, msg []byte) {
	var req wsIdReq
	err := json.Unmarshal(msg, &req)
	if err != nil {
		logger.Warn("could not unmarshal message delete request", "err", err)
		return
	}
	if ctx.User == nil || !ctx.User.IsAdminOfCourse(*ctx.Course) {
		return
	}
	err = r.ChatDao.DeleteChat(req.Id)
	if err != nil {
		logger.Error("could not delete chat", "err", err)
	}
	broadcast := gin.H{
		"delete": req.Id,
	}
	broadcastBytes, err := json.Marshal(broadcast)
	if err != nil {
		logger.Error("could not marshal delete message", "err", err)
		return
	}
	broadcastStream(ctx.Stream.ID, broadcastBytes)
}

func (r chatRoutes) handleApprove(ctx tools.TUMLiveContext, msg []byte) {
	var req wsIdReq
	err := json.Unmarshal(msg, &req)
	if err != nil {
		logger.Warn("could not unmarshal message approve request", "err", err)
		return
	}
	if ctx.User == nil || !ctx.User.IsAdminOfCourse(*ctx.Course) {
		return
	}

	err = r.ChatDao.ApproveChat(req.Id)
	if err != nil {
		logger.Error("could not approve chat", "err", err)
		return
	}

	/* UserId should be the user who gets the message, to add dynamic user specific flags (e.g. Liked)
	 * to the message payload. In this case the Message is freshly approved so no users should have interacted
	 * with that message so far, so we pass 0 instead of a userId.
	 */
	chat, err := r.ChatDao.GetChat(req.Id, 0)
	if err != nil {
		logger.Error("could not get chat", "err", err)
	}
	broadcast := gin.H{
		"approve": req.Id,
		"chat":    chat,
	}
	broadcastBytes, err := json.Marshal(broadcast)
	if err != nil {
		logger.Error("could not marshal approve message", "err", err)
		return
	}
	broadcastStream(ctx.Stream.ID, broadcastBytes)
}

func (r chatRoutes) handleReactTo(ctx tools.TUMLiveContext, msg []byte) {
	var req wsReactToReq
	err := json.Unmarshal(msg, &req)
	if err != nil {
		logger.Warn("could not unmarshal reactTo request", "err", err)
		return
	}

	if _, isAllowed := allowedReactions[req.Reaction]; !isAllowed {
		logger.Warn("user tried to add illegal reaction")
		return
	}

	err = r.ChatDao.ToggleReaction(ctx.User.ID, req.wsIdReq.Id, ctx.User.Name, req.Reaction)
	if err != nil {
		logger.Error("error reacting to message", "err", err)
		return
	}
	reactions, err := r.ChatDao.GetReactions(req.Id)
	if err != nil {
		logger.Error("error getting num of chat reactions", "err", err)
		return
	}
	broadcast := gin.H{
		"reactions": req.Id,
		"payload":   reactions,
	}
	broadcastBytes, err := json.Marshal(broadcast)
	if err != nil {
		logger.Error("Can't marshal reactions message", "err", err)
		return
	}
	broadcastStream(ctx.Stream.ID, broadcastBytes)
}

func (r chatRoutes) handleRetract(ctx tools.TUMLiveContext, msg []byte) {
	var req wsIdReq
	err := json.Unmarshal(msg, &req)
	if err != nil {
		logger.Warn("could not unmarshal message retract request", "err", err)
		return
	}
	if ctx.User == nil || !ctx.User.IsAdminOfCourse(*ctx.Course) {
		return
	}

	err = r.ChatDao.RetractChat(req.Id)
	if err != nil {
		logger.Error("could not retract chat", "err", err)
		return
	}

	err = r.ChatDao.RemoveReactions(req.Id)
	if err != nil {
		logger.Error("could not remove reactions from chat", "err", err)
		return
	}

	chat, err := r.ChatDao.GetChat(req.Id, 0)
	if err != nil {
		logger.Error("could not get chat", "err", err)
	}
	broadcast := gin.H{
		"retract": req.Id,
		"chat":    chat,
	}
	broadcastBytes, err := json.Marshal(broadcast)
	if err != nil {
		logger.Error("could not marshal retract message", "err", err)
		return
	}
	broadcastStream(ctx.Stream.ID, broadcastBytes)
}

func (r chatRoutes) handleMessage(ctx tools.TUMLiveContext, context *realtime.Context, msg []byte) {
	var chat chatReq
	if err := json.Unmarshal(msg, &chat); err != nil {
		logger.Error("error unmarshalling chat message", "err", err)
		return
	}
	if !ctx.Course.ChatEnabled && !ctx.Stream.ChatEnabled {
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
			Status:        http.StatusBadRequest,
			CustomMessage: "not logged in",
		})
		return
	}
	poll, err := r.ChatDao.GetActivePoll(tumLiveContext.Stream.ID)
	if err != nil && err == gorm.ErrRecordNotFound {
		c.JSON(http.StatusNotFound, nil)
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
				logger.Warn("could not get poll option vote count", "err", err)
			}
		}

		pollOptions = append(pollOptions, option.GetStatsMap(voteCount))
	}

	c.JSON(http.StatusOK, gin.H{
		"active":    true,
		"question":  poll.Question,
		"options":   pollOptions,
		"submitted": submitted,
	})
}

func (r chatRoutes) getPolls(c *gin.Context) {
	tumLiveContext := c.MustGet("TUMLiveContext").(tools.TUMLiveContext)

	if tumLiveContext.User == nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusBadRequest,
			CustomMessage: "not logged in",
		})
		return
	}

	polls, err := r.ChatDao.GetPolls(tumLiveContext.Stream.ID)
	if err != nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusInternalServerError,
			CustomMessage: "can not get past polls",
			Err:           err,
		})
		return
	}

	var response []gin.H
	for _, poll := range polls {
		var pollOptions []gin.H
		for _, option := range poll.PollOptions {
			voteCount, _ := r.ChatDao.GetPollOptionVoteCount(option.ID)
			pollOptions = append(pollOptions, option.GetStatsMap(voteCount))
		}
		response = append(response, gin.H{
			"ID":       poll.ID,
			"question": poll.Question,
			"options":  pollOptions,
		})
	}

	c.JSON(http.StatusOK, response)
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

type wsIdReq struct {
	wsReq
	Id uint `json:"id"`
}

type wsReactToReq struct {
	wsIdReq
	Reaction string `json:"reaction"`
}

type submitPollOptionVote struct {
	wsReq
	PollOptionId uint `json:"pollOptionId"`
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
					s.Stats = append(s.Stats, stat)
					if err := daoWrapper.AddStat(stat); err != nil {
						logger.Error("Saving stat failed", "err", err)
					}
				}
			}
		}
	}
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
			logger.Error("Can't save vod view", "err", err)
		}
	}
}
