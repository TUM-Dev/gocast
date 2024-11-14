package api

import (
	"encoding/json"
	"errors"
	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/model"
	"github.com/TUM-Dev/gocast/tools"
	"github.com/TUM-Dev/gocast/tools/realtime"
	"github.com/getsentry/sentry-go"
	"github.com/gin-gonic/gin"
	"net/http"
	"slices"
	"strconv"
	"sync"
	"time"
)

type StreamReactionRoutes struct {
	dao.DaoWrapper
}

// TODO: This can be modified to allow different reactions for different streams
func (r StreamReactionRoutes) allowedReactions(c *gin.Context) {
	c.JSON(http.StatusOK, tools.Cfg.AllowedReactions)
}

func (r StreamReactionRoutes) addReaction(c *gin.Context) {
	cooldownSeconds := 10

	tumLiveContext := c.MustGet("TUMLiveContext").(tools.TUMLiveContext)
	user := tumLiveContext.User
	stream := tumLiveContext.Stream

	if stream == nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusNotFound,
			CustomMessage: "stream not found",
		})
		return
	}

	course, err := r.DaoWrapper.CoursesDao.GetCourseById(c, stream.CourseID)

	if user == nil || err != nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusInternalServerError,
			CustomMessage: "user or course not found",
		})
		return
	}

	if !user.IsEligibleToWatchCourse(course) {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusForbidden,
			CustomMessage: "user not eligible to watch course",
		})
		return
	}

	type reactionRequest struct {
		Reaction string `json:"reaction"`
	}

	var reaction reactionRequest
	if err := c.ShouldBindJSON(&reaction); err != nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusBadRequest,
			CustomMessage: "can not bind body",
			Err:           err,
		})
		return
	}

	// TODO: This can be modified to allow different reactions for different streams
	if !slices.Contains(tools.Cfg.AllowedReactions, reaction.Reaction) {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusBadRequest,
			CustomMessage: "reaction not allowed",
		})
		return
	}

	lastReaction, _ := r.DaoWrapper.StreamReactionDao.GetLastReactionOfUser(c, user.ID)
	if lastReaction.Reaction != "" && lastReaction.CreatedAt.Add(time.Duration(cooldownSeconds)*time.Second).After(time.Now()) {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusTooManyRequests,
			CustomMessage: "cooldown not over",
		})
		return
	}

	reactionObj := model.StreamReaction{
		Reaction: reaction.Reaction,
		StreamID: stream.ID,
		UserID:   user.ID,
	}

	err = r.DaoWrapper.StreamReactionDao.Create(c, &reactionObj)
	if err != nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusInternalServerError,
			CustomMessage: "can not create reaction",
			Err:           err,
		})
		return
	}
	NotifyAdminsOnReaction(stream.ID, reaction.Reaction)
	c.JSON(http.StatusOK, "")
}

// The part below is used for Realtime Connection to the client

const (
	ReactionUpdateRoomName = "reaction-update"
)

var (
	liveReactionListenerMutex sync.RWMutex
	liveReactionListener      = map[uint]*liveReactionAdminSessionsWrapper{}
)

type liveReactionAdminSessionsWrapper struct {
	sessions []*realtime.Context
	stream   uint
}

func RegisterReactionUpdateRealtimeChannel() {
	RealtimeInstance.RegisterChannel(ReactionUpdateRoomName, realtime.ChannelHandlers{
		OnSubscribe:   reactionUpdateOnSubscribe,
		OnUnsubscribe: reactionUpdateOnUnsubscribe,
		OnMessage:     reactionUpdateSetStream,
	})
}

func reactionUpdateOnUnsubscribe(psc *realtime.Context) {
	ctx, _ := psc.Client.Get("ctx") // get gin context
	foundContext, exists := ctx.(*gin.Context).Get("TUMLiveContext")
	if !exists {
		sentry.CaptureException(errors.New("context should exist but doesn't"))
		return
	}

	tumLiveContext := foundContext.(tools.TUMLiveContext)

	var userId uint = 0
	if tumLiveContext.User != nil {
		userId = tumLiveContext.User.ID
	}

	liveReactionListenerMutex.Lock()
	defer liveReactionListenerMutex.Unlock()
	var newSessions []*realtime.Context
	for _, session := range liveReactionListener[userId].sessions {
		if session != psc {
			newSessions = append(newSessions, session)
		}
	}
	if len(newSessions) == 0 {
		delete(liveReactionListener, userId)
	} else {
		liveReactionListener[userId].sessions = newSessions
	}
}

func reactionUpdateOnSubscribe(psc *realtime.Context) {
	ctx, _ := psc.Client.Get("ctx") // get gin context

	foundContext, exists := ctx.(*gin.Context).Get("TUMLiveContext")
	if !exists {
		sentry.CaptureException(errors.New("context should exist but doesn't"))
		return
	}

	tumLiveContext := foundContext.(tools.TUMLiveContext)

	var userId uint = 0
	var err error

	if tumLiveContext.User != nil {
		userId = tumLiveContext.User.ID
	} else {
		logger.Error("could not fetch public courses", "err", err)
		return

	}

	liveReactionListenerMutex.Lock()
	if liveReactionListener[userId] != nil {
		liveReactionListener[userId] = &liveReactionAdminSessionsWrapper{append(liveUpdateListener[userId].sessions, psc), liveReactionListener[userId].stream}
	} else {
		liveReactionListener[userId] = &liveReactionAdminSessionsWrapper{[]*realtime.Context{psc}, 0}
	}
	liveReactionListenerMutex.Unlock()
}

func reactionUpdateSetStream(psc *realtime.Context, message *realtime.Message) {
	logger.Info("reactionUpdateSetStream", "message", string(message.Payload))
	ctx, _ := psc.Client.Get("ctx") // get gin context

	foundContext, exists := ctx.(*gin.Context).Get("TUMLiveContext")
	if !exists {
		sentry.CaptureException(errors.New("context should exist but doesn't"))
		return
	}

	tumLiveContext := foundContext.(tools.TUMLiveContext)

	var userId uint = 0
	var err error

	if tumLiveContext.User != nil {
		userId = tumLiveContext.User.ID
	} else {
		logger.Error("could not get user from request", "err", err)
		return
	}

	type Message struct {
		StreamID string `json:"streamId"`
	}

	var messageObj Message
	err = json.Unmarshal(message.Payload, &messageObj)

	if err != nil {
		logger.Error("could not unmarshal message", "err", err)
		return
	}

	liveReactionListenerMutex.Lock()
	if liveReactionListener[userId] != nil {
		uId, err := strconv.Atoi(messageObj.StreamID)
		if err != nil {
			logger.Error("could not convert streamID to int", "err", err)
			return
		}
		liveReactionListener[userId].stream = uint(uId)
	} else {
		logger.Error("User has no live reaction listener")
	}
	liveReactionListenerMutex.Unlock()
}

func NotifyAdminsOnReaction(streamID uint, reaction string) {
	liveReactionListenerMutex.Lock()
	reactionStruct := struct {
		Reaction string `json:"reaction"`
	}{
		Reaction: reaction,
	}
	reactionMarshaled, err := json.Marshal(reactionStruct)
	if err != nil {
		logger.Error("could not marshal reaction", "err", err)
		return
	}
	for _, session := range liveReactionListener {
		if session.stream == streamID {
			for _, s := range session.sessions {
				err := s.Send([]byte(reactionMarshaled))
				if err != nil {
					logger.Error("can't write reaction to session", "err", err)
				}
			}
		}
	}
}
