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
	UpdateTypeCourseWentLive = "course_went_live"
)

var liveMelody *melody.Melody

var liveUpdateListenerMutex sync.RWMutex
var liveUpdateListener = map[uint]*liveUpdateUserSessionsWrapper{}

const maxLiveUpdateParticipants = 10000

type liveUpdateRoutes struct {
	dao.DaoWrapper
}

type liveUpdateUserSessionsWrapper struct {
	sessions []*melody.Session
	courses  []uint
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
	ctxMap["dao"] = r.DaoWrapper

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

	var userId uint = 0
	if tumLiveContext.User == nil {
		userId = tumLiveContext.User.ID
	}

	liveUpdateListenerMutex.Lock()
	var newSessions []*melody.Session
	for _, session := range liveUpdateListener[userId].sessions {
		if session != s {
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

var liveUpdateConnectionHandler = func(s *melody.Session) {
	ctx, _ := s.Get("ctx") // get gin context
	daoWrapper, _ := s.Get("dao")

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
		liveUpdateListener[userId] = &liveUpdateUserSessionsWrapper{append(liveUpdateListener[userId].sessions, s), courses}
	} else {
		liveUpdateListener[userId] = &liveUpdateUserSessionsWrapper{[]*melody.Session{s}, courses}
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
