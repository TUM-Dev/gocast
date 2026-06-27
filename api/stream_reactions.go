package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"strconv"
	"sync"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/gin-gonic/gin"

	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/model"
	"github.com/TUM-Dev/gocast/tools"
	"github.com/TUM-Dev/gocast/tools/realtime"
)

type StreamReactionRoutes struct {
	dao.DaoWrapper
}

// TODO: This can be modified to allow different reactions for different streams
func (r StreamReactionRoutes) allowedReactions(c *gin.Context) {
	c.JSON(http.StatusOK, tools.Cfg.AllowedReactions)
}

func (r StreamReactionRoutes) addReaction(c *gin.Context) {
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

	if user == nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusUnauthorized,
			CustomMessage: "user not authenticated",
		})
		return
	}

	if err != nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusInternalServerError,
			CustomMessage: "course lookup failed",
			Err:           err,
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

	// This can be modified to allow different reactions for different streams
	if !slices.Contains(tools.Cfg.AllowedReactions, reaction.Reaction) {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusBadRequest,
			CustomMessage: "reaction not allowed",
		})
		return
	}

	lastReaction, _ := r.DaoWrapper.StreamReactionDao.GetLastReactionOfUser(c, user.ID)
	// This contains the cooldown logic, to change this value change the time.Duration(10) to the desired cooldown time
	if lastReaction.Reaction != "" && lastReaction.CreatedAt.Add(time.Duration(10)*time.Second).After(time.Now()) {
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
	reactionDaoWrapper        dao.DaoWrapper
)

type liveReactionAdminSessionsWrapper struct {
	sessions []*realtime.Context
	stream   uint
}

func collectSessionsForStream(streamID uint) []*realtime.Context {
	liveReactionListenerMutex.RLock()
	defer liveReactionListenerMutex.RUnlock()

	targets := make([]*realtime.Context, 0)
	for _, listener := range liveReactionListener {
		if listener.stream != streamID {
			continue
		}
		targets = append(targets, listener.sessions...)
	}

	return targets
}

func collectSessionsGroupedByStream() map[uint][]*realtime.Context {
	liveReactionListenerMutex.RLock()
	defer liveReactionListenerMutex.RUnlock()

	targetsByStream := make(map[uint][]*realtime.Context)
	for _, listener := range liveReactionListener {
		if listener.stream == 0 {
			continue
		}
		targetsByStream[listener.stream] = append(targetsByStream[listener.stream], listener.sessions...)
	}

	return targetsByStream
}

func pruneReactionSessions(dead map[*realtime.Context]struct{}) {
	if len(dead) == 0 {
		return
	}

	liveReactionListenerMutex.Lock()
	defer liveReactionListenerMutex.Unlock()

	for userID, listener := range liveReactionListener {
		filtered := listener.sessions[:0]
		for _, session := range listener.sessions {
			if _, remove := dead[session]; remove {
				continue
			}
			filtered = append(filtered, session)
		}

		if len(filtered) == 0 {
			delete(liveReactionListener, userID)
			continue
		}

		listener.sessions = filtered
	}
}

func RegisterReactionUpdateRealtimeChannel(wrapper dao.DaoWrapper) {
	reactionDaoWrapper = wrapper

	RealtimeInstance.RegisterChannel(
		ReactionUpdateRoomName,
		realtime.ChannelHandlers{
			OnSubscribe:   reactionUpdateOnSubscribe,
			OnUnsubscribe: reactionUpdateOnUnsubscribe,
			OnMessage:     reactionUpdateSetStream,
		},
	)

	go func() {
		// Notify admins every 5 seconds
		logger.Info("Starting periodic notification of reaction percentages")
		for {
			time.Sleep(5 * time.Second)
			NotifyAdminsOnReactionPercentages(context.Background())
		}
	}()
}

func reactionUpdateOnUnsubscribe(psc *realtime.Context) {
	logger.Debug("Unsubscribing from reaction Update")
	ctx, _ := psc.Client.Get("ctx") // get gin context
	foundContext, exists := ctx.(*gin.Context).Get("TUMLiveContext")
	if !exists {
		sentry.CaptureException(errors.New("context should exist but doesn't"))
		return
	}

	tumLiveContext := foundContext.(tools.TUMLiveContext)

	var userId uint
	if tumLiveContext.User != nil {
		userId = tumLiveContext.User.ID
	}

	liveReactionListenerMutex.Lock()
	defer liveReactionListenerMutex.Unlock()
	listener := liveReactionListener[userId]
	if listener == nil {
		return
	}
	var newSessions []*realtime.Context
	for _, session := range listener.sessions {
		if session != psc {
			newSessions = append(newSessions, session)
		}
	}
	if len(newSessions) == 0 {
		delete(liveReactionListener, userId)
	} else {
		listener.sessions = newSessions
	}
	logger.Debug("Successfully unsubscribed from reaction Update")
}

func reactionUpdateOnSubscribe(psc *realtime.Context) {
	ctx, _ := psc.Client.Get("ctx") // get gin context

	foundContext, exists := ctx.(*gin.Context).Get("TUMLiveContext")
	if !exists {
		sentry.CaptureException(errors.New("context should exist but doesn't"))
		return
	}

	tumLiveContext := foundContext.(tools.TUMLiveContext)

	var userId uint
	var err error

	if tumLiveContext.User != nil {
		userId = tumLiveContext.User.ID
	} else {
		logger.Error("could not fetch public courses", "err", err)
		return

	}

	liveReactionListenerMutex.Lock()
	defer liveReactionListenerMutex.Unlock()
	existing := liveReactionListener[userId]
	if existing != nil {
		liveReactionListener[userId] = &liveReactionAdminSessionsWrapper{append(existing.sessions, psc), liveReactionListener[userId].stream}
	} else {
		liveReactionListener[userId] = &liveReactionAdminSessionsWrapper{[]*realtime.Context{psc}, 0}
	}
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

	var userId uint
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

	stream, err := reactionDaoWrapper.StreamsDao.GetStreamByID(context.TODO(), messageObj.StreamID)
	if err != nil {
		logger.Error("Cant get stream by id", "err", err)
		return
	}
	course, err := reactionDaoWrapper.CoursesDao.GetCourseById(context.TODO(), stream.CourseID)
	if err != nil {
		logger.Error("Cant get course by id", "err", err)
		return
	}
	if !tumLiveContext.User.IsAdminOfCourse(course) {
		logger.Error("User is not admin of course")
		reactionUpdateOnUnsubscribe(psc)
		return
	}

	liveReactionListenerMutex.Lock()
	defer liveReactionListenerMutex.Unlock()
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
}

func NotifyAdminsOnReaction(streamID uint, reaction string) {
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

	targets := collectSessionsForStream(streamID)
	dead := make(map[*realtime.Context]struct{})
	for _, session := range targets {
		err := session.Send(reactionMarshaled)
		if err != nil {
			logger.Error("can't write reaction to session", "err", err)
			dead[session] = struct{}{}
		}
	}

	pruneReactionSessions(dead)
}

func NotifyAdminsOnReactionPercentages(context context.Context) {
	targetsByStream := collectSessionsGroupedByStream()

	streamReactionPercentages := map[uint]map[string]float64{}

	for stream := range targetsByStream {
		reactionsRaw, err := daoWrapper.StreamReactionDao.GetByStreamWithinMinutes(context, stream, 2) // TODO: Make this variable for the lecturer
		if err != nil {
			logger.Error("could not get reactions for stream", "stream", stream, "err", err)
			return
		}

		reactions := make(map[string]int)
		for _, reaction := range reactionsRaw {
			reactions[reaction.Reaction]++
		}

		totalReactions := 0
		for _, count := range reactions {
			totalReactions += count
		}
		if totalReactions == 0 {
			// logger.Debug("no reactions for stream", "stream", stream)
			continue
		}

		streamReactionPercentages[stream] = make(map[string]float64)
		for reaction, count := range reactions {
			streamReactionPercentages[stream][reaction] = float64(count) / float64(totalReactions)
		}
	}

	dead := make(map[*realtime.Context]struct{})
	for streamID, sessions := range targetsByStream {
		reactionPercentages := streamReactionPercentages[streamID]
		reactionPercentagesMarshaled, err := json.Marshal(reactionPercentages)
		if err != nil {
			logger.Error("could not marshal reaction percentages", "err", err)
			return
		}
		for _, session := range sessions {
			err := session.Send([]byte("{\"percentages\": " + string(reactionPercentagesMarshaled) + "}"))
			if err != nil {
				logger.Error("can't write reaction percentages to session", "err", err)
				dead[session] = struct{}{}
			}
		}
	}

	pruneReactionSessions(dead)
}
