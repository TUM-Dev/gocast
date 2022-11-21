package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/getsentry/sentry-go"
	"github.com/gin-gonic/gin"
	"github.com/joschahenningsen/TUM-Live/dao"
	"github.com/joschahenningsen/TUM-Live/tools"
	"github.com/joschahenningsen/TUM-Live/tools/realtime"
	log "github.com/sirupsen/logrus"
	"strconv"
	"strings"
	"sync"
)

var wsMapLock sync.RWMutex

var sessionsMap = map[uint][]*sessionWrapper{}

const (
	TypeServerErr = "error"
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
		log.WithError(err).Error("can't write initial stats to session")
	}
}

// sendServerMessage sends a server message to the client(s)
func sendServerMessage(msg string, t string, sessions ...*realtime.Context) {
	msgBytes, _ := json.Marshal(gin.H{"server": msg, "type": t})
	for _, session := range sessions {
		err := session.Send(msgBytes)
		if err != nil {
			log.WithError(err).Error("can't write server message to session")
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
