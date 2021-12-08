package api

import (
	"TUM-Live/dao"
	"TUM-Live/tools"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gabstv/melody"
	"github.com/getsentry/sentry-go"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"sync"
	"time"
)

var wsMapLock sync.RWMutex

var sessionsMap = map[uint][]*melody.Session{}

const (
	TypeServerInfo = "info"
	TypeServerWarn = "warn"
	TypeServerErr  = "error"
)

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
	var uid uint = 0
	if tumLiveContext.User != nil {
		uid = tumLiveContext.User.ID
	}
	if tumLiveContext.Course.ChatEnabled {
		sendServerMessageWithBackoff(s, uid, tumLiveContext.Stream.ID, "Welcome to the chatroom! Please be nice to each other and stay on topic if you want this feature to stay active.", TypeServerInfo)
	}
	if !tumLiveContext.Course.AnonymousChatEnabled {
		sendServerMessageWithBackoff(s, uid, tumLiveContext.Stream.ID, "The Broadcaster disabled anonymous messaging for this room.", TypeServerWarn)
	}
}

// sendServerMessageWithBackoff sends a message to the client(if it didn't send a message to this user in the last 10 Minutes and the client is logged in)
func sendServerMessageWithBackoff(session *melody.Session, userId uint, streamId uint, msg string, t string) {
	if userId == 0 {
		return
	}
	cacheKey := fmt.Sprintf("shouldSendServerMsg_%d_%d", userId, streamId)
	// if the user has sent a message in the last 10 Minutes, don't send a message
	_, shouldSkip := tools.GetCacheItem(cacheKey)
	if shouldSkip {
		return
	}
	msgBytes, _ := json.Marshal(gin.H{"server": msg, "type": t})
	err := session.Write(msgBytes)
	if err != nil {
		log.WithError(err).Error("can't write server message to session")
	}
	// set cache item with ttl, so the user won't get a message for 10 Minutes
	tools.SetCacheItem(cacheKey, true, time.Minute*10)
}

//sendServerMessage sends a server message to the client
func sendServerMessage(session *melody.Session, msg string, t string) {
	msgBytes, _ := json.Marshal(gin.H{"server": msg, "type": t})
	err := session.Write(msgBytes)
	if err != nil {
		log.WithError(err).Error("can't write server message to session")
	}
}

func BroadcastStats() {
	for sID, sessions := range sessionsMap {
		if len(sessions) == 0 {
			continue
		}
		stream, err := dao.GetStreamByID(context.Background(), fmt.Sprintf("%d", sID))
		if err != nil || stream.Recording {
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
