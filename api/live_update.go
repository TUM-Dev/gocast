package api

import (
	"errors"
	"github.com/gabstv/melody"
	"github.com/getsentry/sentry-go"
	"github.com/gin-gonic/gin"
	"github.com/joschahenningsen/TUM-Live/dao"
	"github.com/joschahenningsen/TUM-Live/model"
	"github.com/joschahenningsen/TUM-Live/tools"
	log "github.com/sirupsen/logrus"
	"sync"
)

var liveMelody *melody.Melody

var liveUpdateListenerMutex sync.RWMutex
var liveUpdateListener = map[uint]*liveUpdateSessionWrapper{}

const maxLiveUpdateParticipants = 10000

type liveUpdateRoutes struct {
	dao.DaoWrapper
}

type liveUpdateSessionWrapper struct {
	sessions []*melody.Session
	courses  *[]model.Course
}

func configGinLiveUpdateRouter(router *gin.RouterGroup, daoWrapper dao.DaoWrapper) {
	routes := liveUpdateRoutes{daoWrapper}

	router.GET("/ws", routes.handleLiveConnect)

	if liveMelody == nil {
		log.Printf("creating liveMelody")
		liveMelody = melody.New()
	}

	liveMelody.HandleConnect(liveUpdateConnectionHandler)
	liveMelody.HandleDisconnect(liveUpdateDisconnectHandler)
}

func (r liveUpdateRoutes) handleLiveConnect(c *gin.Context) {
	// max participants in chat to prevent attacks
	if liveMelody.Len() > maxLiveUpdateParticipants {
		return
	}

	ctxMap := make(map[string]interface{}, 1)
	ctxMap["ctx"] = c
	ctxMap["ctx"] = c

	_ = liveMelody.HandleRequestWithKeys(c.Writer, c.Request, ctxMap)
}

func liveUpdateDisconnectHandler(s *melody.Session) {
	ctx, _ := s.Get("ctx") // get gin context
	foundContext, exists := ctx.(*gin.Context).Get("TUMLiveContext")
	if !exists {
		sentry.CaptureException(errors.New("context should exist but doesn't"))
		return
	}
	tumLiveContext := foundContext.(tools.TUMLiveContext)
	if tumLiveContext.User == nil {
		return
	}

	liveUpdateListenerMutex.Lock()
	var newSessions []*melody.Session
	for _, session := range liveUpdateListener[tumLiveContext.User.ID].sessions {
		if session != s {
			newSessions = append(newSessions, session)
		}
	}
	if len(newSessions) == 0 {
		delete(liveUpdateListener, tumLiveContext.User.ID)
	} else {
		liveUpdateListener[tumLiveContext.User.ID].sessions = newSessions
	}
	liveUpdateListenerMutex.Unlock()
}

var liveUpdateConnectionHandler = func(s *melody.Session) {
	ctx, _ := s.Get("ctx") // get gin context
	foundContext, exists := ctx.(*gin.Context).Get("TUMLiveContext")
	if !exists {
		sentry.CaptureException(errors.New("context should exist but doesn't"))
		return
	}
	tumLiveContext := foundContext.(tools.TUMLiveContext)
	if tumLiveContext.User == nil {
		log.Error("need to be logged in to connect to live updates")
		return
	}

	courses := &tumLiveContext.User.Courses

	liveUpdateListenerMutex.Lock()
	if liveUpdateListener[tumLiveContext.User.ID] != nil {
		liveUpdateListener[tumLiveContext.User.ID] = &liveUpdateSessionWrapper{append(liveUpdateListener[tumLiveContext.User.ID].sessions, s), courses}
	} else {
		liveUpdateListener[tumLiveContext.User.ID] = &liveUpdateSessionWrapper{[]*melody.Session{s}, courses}
	}
	liveUpdateListenerMutex.Unlock()
}
