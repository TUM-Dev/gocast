package api

import (
	"encoding/json"
	"errors"
	"slices"
	"sync"

	"github.com/gin-gonic/gin"

	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/model"
	"github.com/TUM-Dev/gocast/tools"
	"github.com/TUM-Dev/gocast/tools/realtime"
)

const (
	// LiveRunnerPageUpdateRoomName is the name of the room for live runner page updates
	LiveRunnerPageUpdateRoomName = "live-runner-page-update"
)

var (
	liveRunnerPageUpdateListenerMutex sync.RWMutex
	liveRunnerPageUpdateListener      map[uint]*liveRunnerPageUpdateSessionsWrapper
	// TODO: Refactor
	daoWrapper dao.DaoWrapper
)

type liveRunnerPageUpdateSessionsWrapper struct {
	sessions []*realtime.Context
}

func RegisterLiveRunnerPageUpdateRealtimeChannel(wrapper dao.DaoWrapper) {
	RealtimeInstance.RegisterChannel(LiveRunnerPageUpdateRoomName, realtime.ChannelHandlers{
		OnSubscribe:   liveRunnerPageUpdateOnSubscribe,
		OnUnsubscribe: liveRunnerPageUpdateOnUnsubscribe,
		OnMessage:     liveRunnerPageUpdateOnMessage,
	})
	daoWrapper = wrapper
	liveRunnerPageUpdateListener = make(map[uint]*liveRunnerPageUpdateSessionsWrapper)
}

func liveRunnerPageUpdateOnUnsubscribe(psc *realtime.Context) {
	if psc == nil {
		logger.Error("Message or context is nil")
		return
	}
	logger.Debug("Unsubscribing from live runner page update channel", "user", psc.Client.Id)
	ctx, _ := psc.Client.Get("ctx") // get gin context
	foundContext, exists := ctx.(*gin.Context).Get("TUMLiveContext")

	if !exists {
		logger.Error("context should exist but doesn't")
		return
	}
	tumLiveContext := foundContext.(tools.TUMLiveContext)

	var userId uint
	if tumLiveContext.User != nil {
		userId = tumLiveContext.User.ID
	}

	liveRunnerPageUpdateListenerMutex.Lock()
	defer liveRunnerPageUpdateListenerMutex.Unlock()

	// Subscribe registers the subscriber before running OnSubscribe, and OnSubscribe
	// returns without registering for every non-admin, so the entry may not exist.
	listener, ok := liveRunnerPageUpdateListener[userId]
	if !ok {
		logger.Debug("no live runner update subscription to remove", "user", psc.Client.Id)
		return
	}

	var newSessions []*realtime.Context
	for _, session := range listener.sessions {
		if session != psc {
			newSessions = append(newSessions, session)
		}
	}
	if len(newSessions) == 0 {
		delete(liveRunnerPageUpdateListener, userId)
	} else {
		listener.sessions = newSessions
	}
	logger.Debug("Successfully unsubscribed from live runner updates")
}

func liveRunnerPageUpdateOnSubscribe(psc *realtime.Context) {
	if psc == nil {
		logger.Error("Message or context is nil")
		return
	}

	ctx, _ := psc.Client.Get("ctx") // get gin context

	foundContext, exists := ctx.(*gin.Context).Get("TUMLiveContext")
	if !exists {
		logger.Error("context should exist but doesn't")
		return
	}

	tumLiveContext := foundContext.(tools.TUMLiveContext)

	var userId uint
	var err error

	if tumLiveContext.User != nil {
		userId = tumLiveContext.User.ID
		if !tumLiveContext.User.Can(model.PermAdministerServer) {
			err = errors.New("user is not admin")
			logger.Error("User is not admin", "err", err)
			return
		}
	} else {
		logger.Error("User ID not found, cannot verify admin status", "err", err)
		return
	}

	liveRunnerPageUpdateListenerMutex.Lock()
	defer liveRunnerPageUpdateListenerMutex.Unlock()
	existing := liveUpdateListener[userId]
	if existing != nil {
		liveRunnerPageUpdateListener[userId] = &liveRunnerPageUpdateSessionsWrapper{append(existing.sessions, psc)}
	} else {
		liveRunnerPageUpdateListener[userId] = &liveRunnerPageUpdateSessionsWrapper{[]*realtime.Context{psc}}
	}
}

func sendMessageToSession(session *realtime.Context, message interface{}) {
	if message == nil {
		logger.Error("Message or context is nil")
		return
	}
	messageMarshalled, err := json.Marshal(message)
	if err != nil {
		logger.Error("Could not marshal message", "err", err)
		return
	}

	err = session.Send(messageMarshalled)
	if err != nil {
		logger.Error("Cannot send message to client", "err", err)
	}
}

func liveRunnerPageUpdateOnMessage(psc *realtime.Context, message *realtime.Message) {
	if message == nil || psc == nil {
		logger.Error("Message or context is nil")
		return
	}
	// logger.Info("Received message on live runner page update channel", "message", message.Payload)
	ctx, exists := psc.Client.Get("ctx") // get gin context
	if !exists {
		logger.Error("Could not get context from client")
		return
	}
	foundContext, exists := ctx.(*gin.Context).Get("TUMLiveContext")
	if !exists {
		logger.Error("context should exist but doesn't")
		return
	}
	tumLiveContext := foundContext.(tools.TUMLiveContext)
	var userId uint
	if tumLiveContext.User != nil {
		userId = tumLiveContext.User.ID
	} else {
		logger.Error("User ID not found", "err", errors.New("user ID not found"))
		return
	}
	if liveRunnerPageUpdateListener[userId] != nil {
		if liveRunnerPageUpdateListener[userId].sessions != nil {
			if slices.Contains(liveRunnerPageUpdateListener[userId].sessions, psc) {
				msgMap := make(map[string]interface{})
				err := json.Unmarshal(message.Payload, &msgMap)
				if err != nil {
					logger.Error("Could not unmarshal message", "err", err)
				}

				switch msgMap["task"] {
				case "aliveStatusUpdate":
					doAliveStatusUpdate(psc)
				default:
					logger.Error("Unknown message type", "type", msgMap["task"])
				}
				return
			}
		}
	}
	logger.Error("User not subscribed to live runner page update channel", "userId", userId)
}

func doAliveStatusUpdate(psc *realtime.Context) {
	type aliveStatus struct {
		Runner   string `json:"runner"`
		Status   bool   `json:"status"`
		JobCount uint64 `json:"jobCount"`
	}

	ctx, _ := psc.Client.Get("ctx") // get gin context
	foundContext, exists := ctx.(*gin.Context)
	if !exists {
		logger.Error("context should exist but doesn't")
		return
	}

	var statuses []aliveStatus
	runners, err := daoWrapper.RunnerDao.GetAll(foundContext)
	if err != nil {
		logger.Error("Could not get runners", "err", err)
		return
	}
	for _, runner := range runners {
		status := aliveStatus{
			Runner:   runner.Hostname,
			Status:   runner.Alive(),
			JobCount: runner.JobCount,
		}
		statuses = append(statuses, status)
	}

	msg := struct {
		Task     string        `json:"task"`
		Statuses []aliveStatus `json:"statuses"`
	}{
		Task:     "aliveStatusUpdate",
		Statuses: statuses,
	}
	sendMessageToSession(psc, msg)
}

// ----------------------------------------------- REST API CALLS ---------------------------------------------

type runnerRoutes struct {
	dao dao.RunnerDao
}

func configRunnerRouter(r *gin.Engine, daoWrapper dao.DaoWrapper) {
	g := r.Group("/api/runners")
	g.Use(tools.RequirePermission(model.PermAdministerServer))

	routes := runnerRoutes{dao: daoWrapper.RunnerDao}

	g.DELETE("/:hostname", routes.DeleteRunner)
}

func (r *runnerRoutes) DeleteRunner(c *gin.Context) {
	hostname := c.Param("hostname")
	err := r.dao.Delete(c, hostname)
	if err != nil {
		logger.Error("can not delete runner", "err", err)
		_ = c.Error(tools.RequestError{
			Status:        500,
			CustomMessage: "can not delete runner",
			Err:           err,
		})
		return
	}
	c.JSON(200, gin.H{"status": "ok"})
}
