package api

import (
	"encoding/json"
	"errors"
	"github.com/RBG-TUM/commons"
	"github.com/gabstv/melody"
	"github.com/getsentry/sentry-go"
	"github.com/gin-gonic/gin"
	"github.com/joschahenningsen/TUM-Live/dao"
	"github.com/joschahenningsen/TUM-Live/model"
	"github.com/joschahenningsen/TUM-Live/tools"
	"github.com/joschahenningsen/TUM-Live/tools/tum"
	log "github.com/sirupsen/logrus"
	"sync"
)

const (
	LiveUpdatePubSubRoomName = "live-update"
	UpdateTypeCourseWentLive = "course_went_live"
)

var liveUpdateListenerMutex sync.RWMutex
var liveUpdateListener = map[uint]*liveUpdateUserSessionsWrapper{}

type liveUpdateUserSessionsWrapper struct {
	sessions []*melody.Session
	courses  []uint
}

func RegisterLiveUpdatePubSubChannel() {
	RegisterPubSubChannel(LiveUpdatePubSubRoomName, PubSubMessageHandlers{
		onSubscribe:   liveUpdateOnSubscribe,
		onUnsubscribe: liveUpdateOnUnsubscribe,
	})
}

func liveUpdateOnUnsubscribe(psc *PubSubContext) {
	ctx, _ := psc.Get("ctx") // get gin context
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

	liveUpdateListenerMutex.Lock()
	var newSessions []*melody.Session
	for _, session := range liveUpdateListener[userId].sessions {
		if session != psc.Client.session {
			newSessions = append(newSessions, session)
		}
	}
	if len(newSessions) == 0 {
		delete(liveUpdateListener, userId)
	} else {
		liveUpdateListener[userId].sessions = newSessions
	}
	liveUpdateListenerMutex.Unlock()
}

func liveUpdateOnSubscribe(psc *PubSubContext) {
	ctx, _ := psc.Get("ctx") // get gin context
	daoWrapper, _ := psc.Get("dao")

	foundContext, exists := ctx.(*gin.Context).Get("TUMLiveContext")
	if !exists {
		sentry.CaptureException(errors.New("context should exist but doesn't"))
		return
	}

	tumLiveContext := foundContext.(tools.TUMLiveContext)

	var userCourses []model.Course
	var userId uint = 0
	var err error
	year, term := tum.GetCurrentSemester()

	if tumLiveContext.User != nil {
		userId = tumLiveContext.User.ID
		if userCourses, err = daoWrapper.(dao.DaoWrapper).CoursesDao.GetPublicAndLoggedInCourses(year, term); err != nil {
			log.WithError(err).Error("could not fetch public and logged in courses")
			return
		}
		userCourses = commons.Unique(userCourses, func(c model.Course) uint { return c.ID })
	} else {
		if userCourses, err = daoWrapper.(dao.DaoWrapper).CoursesDao.GetPublicCourses(year, term); err != nil {
			log.WithError(err).Error("could not fetch public courses")
			return
		}
	}

	var courses []uint
	for _, course := range userCourses {
		courses = append(courses, course.ID)
	}

	liveUpdateListenerMutex.Lock()
	if liveUpdateListener[userId] != nil {
		liveUpdateListener[userId] = &liveUpdateUserSessionsWrapper{append(liveUpdateListener[userId].sessions, psc.Client.session), courses}
	} else {
		liveUpdateListener[userId] = &liveUpdateUserSessionsWrapper{[]*melody.Session{psc.Client.session}, courses}
	}
	liveUpdateListenerMutex.Unlock()
}

func NotifyLiveUpdateCourseWentLive(courseId uint) {
	updateMessage, _ := json.Marshal(gin.H{"type": UpdateTypeCourseWentLive, "data": gin.H{"courseId": courseId}})
	liveUpdateListenerMutex.Lock()
	for _, userWrap := range liveUpdateListener {
		for _, course := range userWrap.courses {
			if course == courseId {
				for _, session := range userWrap.sessions {
					_ = session.Write(updateMessage)
				}
				break
			}
		}
	}
	liveUpdateListenerMutex.Unlock()
}
