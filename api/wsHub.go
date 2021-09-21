package api

import (
	"TUM-Live/tools"
	"encoding/json"
	"errors"
	"github.com/gabstv/melody"
	"github.com/getsentry/sentry-go"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"sync"
)

var wsMapLock sync.RWMutex

var sessionsMap = map[uint][]*melody.Session{}

var connHandler = func(s *melody.Session) {
	ctx, _ := s.Get("ctx") // get gin context
	foundContext, exists := ctx.(*gin.Context).Get("TUMLiveContext")
	if !exists {
		sentry.CaptureException(errors.New("context should exist but doesn't"))
		return
	}
	tumLiveContext := foundContext.(tools.TUMLiveContext)
	wsMapLock.Lock()
	sessionsMap[tumLiveContext.Stream.ID] = append(sessionsMap[tumLiveContext.Stream.ID], s)
	wsMapLock.Unlock()
	msg, _ := json.Marshal(gin.H{"viewers": len(sessionsMap[tumLiveContext.Stream.ID])})
	err := s.Write(msg)
	if err != nil {
		log.WithError(err).Error("can't write initial stats to session")
	}
}

func BroadcastStats() {
	for sID, sessions := range sessionsMap {
		if len(sessions) == 0 {
			continue
		}
		msg, _ := json.Marshal(gin.H{"viewers": len(sessions)})
		broadcastStream(sID, msg)
	}
}

func broadcastStream(streamID uint, msg []byte) {
	sessions, f := sessionsMap[streamID]
	if !f {
		return
	}
	var newSessions []*melody.Session
	wsMapLock.Lock()
	for _, session := range sessions {
		if !session.IsClosed() {
			newSessions = append(newSessions, session)
		}
	}
	sessionsMap[streamID] = newSessions
	wsMapLock.Unlock()
	for _, session := range newSessions {
		_ = session.Write(msg) // ignore "session closed" error, nothing we can do about it at this point
	}
}
