package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/tools"
	"github.com/TUM-Dev/gocast/tools/realtime"
	"github.com/getsentry/sentry-go"
	"github.com/gin-gonic/gin"
)

var wsMapLock sync.RWMutex

var sessionsMap = map[uint][]*sessionWrapper{}

const (
	TypeServerInfo = "info"
	TypeServerWarn = "warn"
	TypeServerErr  = "error"
)

type sessionWrapper struct {
	session         *realtime.Context
	isAdminOfCourse bool
}

var connHandler = func(context *realtime.Context) {
	foundContext, exists := context.Get("TUMLiveContext") // get gin context
	if !exists {
		sentry.CaptureException(errors.New("context should exist but doesn't"))
		return
	}
	tumLiveContext := foundContext.(tools.TUMLiveContext)
	isAdmin := false
	if tumLiveContext.User != nil {
		isAdmin = tumLiveContext.User.IsAdminOfCourse(*tumLiveContext.Course)
	}
	sessionData := sessionWrapper{context, isAdmin}

	wsMapLock.Lock()
	sessionsMap[tumLiveContext.Stream.ID] = append(sessionsMap[tumLiveContext.Stream.ID], &sessionData)
	wsMapLock.Unlock()

	msg, _ := json.Marshal(gin.H{"viewers": len(sessionsMap[tumLiveContext.Stream.ID])})
	err := context.Send(msg)
	if err != nil {
		logger.Error("can't write initial stats to session", "err", err)
	}
}

// sendServerMessageWithBackoff sends a message to the client(if it didn't send a message to this user in the last 10 Minutes and the client is logged in)
//
//lint:ignore U1000 Ignore unused function
func sendServerMessageWithBackoff(session *realtime.Context, userId uint, streamId uint, msg string, t string) {
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
	err := session.Send(msgBytes)
	if err != nil {
		logger.Error("can't write server message to session", "err", err)
	}
	// set cache item with ttl, so the user won't get a message for 10 Minutes
	tools.SetCacheItem(cacheKey, true, time.Minute*10)
}

// sendServerMessage sends a server message to the client(s)
func sendServerMessage(msg string, t string, sessions ...*realtime.Context) {
	msgBytes, _ := json.Marshal(gin.H{"server": msg, "type": t})
	for _, session := range sessions {
		err := session.Send(msgBytes)
		if err != nil {
			logger.Error("can't write server message to session", "err", err)
		}
	}
}

func BroadcastStats(streamsDao dao.StreamsDao) {
	for sID, sessions := range sessionsMap {
		if len(sessions) == 0 {
			continue
		}
		stream, err := streamsDao.GetStreamByID(context.Background(), fmt.Sprintf("%d", sID))
		if err != nil || stream.Recording {
			continue
		}
		msg, _ := json.Marshal(gin.H{"viewers": len(sessions)})
		broadcastStream(sID, msg)
	}
}

func cleanupSessions() {
	for id, sessions := range sessionsMap {
		roomName := strings.Replace(ChatRoomName, ":streamID", strconv.Itoa(int(id)), -1)
		var newSessions []*sessionWrapper
		for i, session := range sessions {
			if RealtimeInstance.IsSubscribed(roomName, session.session.Client.Id) {
				newSessions = append(newSessions, sessions[i])
			}
		}
		wsMapLock.Lock()
		sessionsMap[id] = newSessions
		wsMapLock.Unlock()
	}
}

func broadcastStream(streamID uint, msg []byte) {
	sessions, f := sessionsMap[streamID]
	if !f {
		return
	}
	wsMapLock.Lock()
	sessions = removeClosed(sessions)
	wsMapLock.Unlock()

	for _, wrapper := range sessions {
		_ = wrapper.session.Send(msg) // ignore "session closed" error, nothing we can do about it at this point
	}
}

func broadcastStreamToAdmins(streamID uint, msg []byte) {
	sessions, f := sessionsMap[streamID]
	if !f {
		return
	}
	wsMapLock.Lock()
	sessions = removeClosed(sessions)
	wsMapLock.Unlock()

	for _, wrapper := range sessions {
		if wrapper.isAdminOfCourse {
			_ = wrapper.session.Send(msg)
		}
	}
}

// removeClosed removes session where IsClosed() is true.
func removeClosed(sessions []*sessionWrapper) []*sessionWrapper {
	var newSessions []*sessionWrapper
	for _, wrapper := range sessions {
		if RealtimeInstance.IsConnected(wrapper.session.Client.Id) {
			newSessions = append(newSessions, wrapper)
		}
	}
	return newSessions
}
