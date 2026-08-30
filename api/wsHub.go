package api

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/tools"
	"github.com/TUM-Dev/gocast/tools/realtime"
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

// sessionsFor returns a snapshot of the sessions watching a stream. Callers must go
// through this rather than reading sessionsMap directly: iterating the map while another
// goroutine writes it is a fatal runtime error, not a recoverable race.
func sessionsFor(streamID uint) []*sessionWrapper {
	wsMapLock.RLock()
	defer wsMapLock.RUnlock()
	return slices.Clone(sessionsMap[streamID])
}

// streamIDsWithSessions returns a snapshot of the streams that currently have sessions.
func streamIDsWithSessions() []uint {
	wsMapLock.RLock()
	defer wsMapLock.RUnlock()
	ids := make([]uint, 0, len(sessionsMap))
	for id, sessions := range sessionsMap {
		if len(sessions) > 0 {
			ids = append(ids, id)
		}
	}
	return ids
}

var connHandler = func(context *realtime.Context) {
	foundContext, exists := context.Get("TUMLiveContext") // get gin context
	if !exists {
		logger.Error("context should exist but doesn't")
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
	viewers := len(sessionsMap[tumLiveContext.Stream.ID])
	wsMapLock.Unlock()

	msg, _ := json.Marshal(gin.H{"viewers": viewers})
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
	for _, sID := range streamIDsWithSessions() {
		sessions := sessionsFor(sID)
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
	for _, id := range streamIDsWithSessions() {
		roomName := strings.ReplaceAll(ChatRoomName, ":streamID", strconv.Itoa(int(id)))
		var newSessions []*sessionWrapper
		for _, session := range sessionsFor(id) {
			if RealtimeInstance.IsSubscribed(roomName, session.session.Client.Id) {
				newSessions = append(newSessions, session)
			}
		}
		wsMapLock.Lock()
		sessionsMap[id] = newSessions
		wsMapLock.Unlock()
	}
}

func broadcastStream(streamID uint, msg []byte) {
	for _, wrapper := range removeClosed(sessionsFor(streamID)) {
		_ = wrapper.session.Send(msg) // ignore "session closed" error, nothing we can do about it at this point
	}
}

func broadcastStreamToAdmins(streamID uint, msg []byte) {
	for _, wrapper := range removeClosed(sessionsFor(streamID)) {
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
