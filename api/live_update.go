package api

import (
	"encoding/json"
	"errors"
	"sync"

	"github.com/RBG-TUM/commons"
	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/model"
	"github.com/TUM-Dev/gocast/tools"
	"github.com/TUM-Dev/gocast/tools/realtime"
	"github.com/TUM-Dev/gocast/tools/tum"
	"github.com/getsentry/sentry-go"
	"github.com/gin-gonic/gin"
)

const (
	LiveUpdateRoomName       = "live-update"
	UpdateTypeCourseWentLive = "course_went_live"
)

var liveUpdateListenerMutex sync.RWMutex
var liveUpdateListener = map[uint]*liveUpdateUserSessionsWrapper{}

type liveUpdateUserSessionsWrapper struct {
	sessions []*realtime.Context
	courses  []uint
}

func RegisterLiveUpdateRealtimeChannel() {
	RealtimeInstance.RegisterChannel(LiveUpdateRoomName, realtime.ChannelHandlers{
		OnSubscribe:   liveUpdateOnSubscribe,
		OnUnsubscribe: liveUpdateOnUnsubscribe,
	})
}

func liveUpdateOnUnsubscribe(psc *realtime.Context) {
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

	liveUpdateListenerMutex.Lock()
	defer liveUpdateListenerMutex.Unlock()
	var newSessions []*realtime.Context
	for _, session := range liveUpdateListener[userId].sessions {
		if session != psc {
			newSessions = append(newSessions, session)
		}
	}
	if len(newSessions) == 0 {
		delete(liveUpdateListener, userId)
	} else {
		liveUpdateListener[userId].sessions = newSessions
	}
}

func liveUpdateOnSubscribe(psc *realtime.Context) {
	ctx, _ := psc.Client.Get("ctx") // get gin context
	daoWrapper, _ := psc.Client.Get("dao")

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
			logger.Error("could not fetch public and logged in courses", "err", err)
			return
		}
		userCourses = commons.Unique(userCourses, func(c model.Course) uint { return c.ID })
	} else {
		if userCourses, err = daoWrapper.(dao.DaoWrapper).CoursesDao.GetPublicCourses(year, term); err != nil {
			logger.Error("could not fetch public courses", "err", err)
			return
		}
	}

	var courses []uint
	for _, course := range userCourses {
		courses = append(courses, course.ID)
	}

	liveUpdateListenerMutex.Lock()
	if liveUpdateListener[userId] != nil {
		liveUpdateListener[userId] = &liveUpdateUserSessionsWrapper{append(liveUpdateListener[userId].sessions, psc), courses}
	} else {
		liveUpdateListener[userId] = &liveUpdateUserSessionsWrapper{[]*realtime.Context{psc}, courses}
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
					_ = session.Send(updateMessage)
				}
				break
			}
		}
	}
	liveUpdateListenerMutex.Unlock()
}
