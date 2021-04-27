package api

import (
	"TUM-Live/dao"
	"TUM-Live/model"
	"TUM-Live/tools"
	"context"
	"encoding/json"
	"fmt"
	"github.com/getsentry/sentry-go"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"gopkg.in/olahol/melody.v1"
	"log"
	"strconv"
	"sync"
	"time"
)

var m *melody.Melody
var stats = map[string]int{}
var statsLock = sync.RWMutex{}

func configGinChatRouter(router gin.IRoutes) {
	router.GET("/:vidId/ws", ChatStream)
	if m == nil {
		log.Printf("creating melody")
		m = melody.New()
	}
	m.HandleConnect(func(s *melody.Session) {
		ctx, _ := s.Get("ctx") // get gin context
		vid := ctx.(*gin.Context).Param("vidId")
		if stream, err := dao.GetStreamByID(context.Background(), vid); err == nil && stream.LiveNow {
			if statMsg, err := json.Marshal(gin.H{"viewers": stats[vid]}); err == nil {
				_ = s.Write(statMsg)
			} else {
				sentry.CaptureException(err)
			}
		}
	})

	m.HandleMessage(func(s *melody.Session, msg []byte) {
		ctx, _ := s.Get("ctx") // get gin context
		var chat ChatReq
		if err := json.Unmarshal(msg, &chat); err != nil {
			return
		}
		if chat.Msg == "" || len(chat.Msg) > 200 {
			return
		}
		user, uErr := tools.GetUser(ctx.(*gin.Context))
		student, sErr := tools.GetStudent(ctx.(*gin.Context))
		var uid string
		if uErr == nil {
			uid = strconv.Itoa(int(user.ID))
		} else if sErr == nil {
			uid = student.ID
		} else {
			return
		}
		if dao.IsUserCooledDown(uid) {
			return
		}
		vID, err := strconv.Atoi(ctx.(*gin.Context).Param("vidId"))
		if err != nil {
			return
		}
		stream, err := dao.GetStreamByID(context.Background(), fmt.Sprintf("%v", vID))
		if err != nil {
			return
		}
		if course, err := dao.GetCourseById(context.Background(), stream.CourseID); err != nil || !course.ChatEnabled {
			return
		}
		session := sessions.Default(ctx.(*gin.Context))
		uname := session.Get("Name").(string)
		if chat.Anonymous {
			uname = "Anonymous"
		}
		dao.AddMessage(model.Chat{
			UserID:   uid,
			UserName: uname,
			Message:  chat.Msg,
			StreamID: uint(vID),
			Admin:    uErr == nil && user.IsAdminOfCourse(stream.CourseID),
			SendTime: time.Now().In(tools.Loc),
		})
		broadcast, err := json.Marshal(ChatRep{
			Msg:   chat.Msg,
			Name:  uname,
			Admin: uErr == nil && user.IsAdminOfCourse(stream.CourseID),
		})
		if err == nil {
			_ = m.BroadcastFilter(broadcast, func(q *melody.Session) bool { // filter broadcasting to same lecture.
				return q.Request.URL.Path == s.Request.URL.Path
			})
		}
	})
}

type ChatReq struct {
	Msg       string `json:"msg"`
	Anonymous bool   `json:"anonymous"`
}
type ChatRep struct {
	Msg   string `json:"msg"`
	Name  string `json:"name"`
	Admin bool   `json:"admin"`
}

func CollectStats() {
	log.Printf("Collecting stats\n")
	defer sentry.Flush(time.Second * 2)
	for sID, numWatchers := range stats {
		log.Printf("Collecting stats for stream %v, viewers:%v\n", sID, numWatchers)
		stat := model.Stat{
			Time:    time.Now(),
			Viewers: numWatchers,
		}
		if s, err := dao.GetStreamByID(context.Background(), sID); err == nil {
			if !s.LiveNow { // collect stats for livestreams only
				log.Printf("stream not live, skipping stats\n")
				delete(stats, strconv.Itoa(int(s.ID)))
				continue
			}
			s.Stats = append(s.Stats, stat)
			if err = dao.SaveStream(&s); err != nil {
				sentry.CaptureException(err)
				log.Printf("Error saving stats: %v\n", err)
			}
			if mStat, err := json.Marshal(gin.H{"viewers": numWatchers}); err == nil {
				_ = m.BroadcastFilter(mStat, func(q *melody.Session) bool { // filter broadcasting to same lecture.
					return q.Request.URL.Path == fmt.Sprintf("/api/chat/%v/ws", sID)
				})
			} else {
				sentry.CaptureException(err)
			}
		}
	}
}

func ChatStream(c *gin.Context) {
	// max participants in chat to prevent attacks
	if m.Len() > 10000 {
		return
	}
	go addUser(c.Param("vidId"))
	joinTime := time.Now()
	defer removeUser(c.Param("vidId"), joinTime)
	ctxMap := make(map[string]interface{}, 1)
	ctxMap["ctx"] = c

	_ = m.HandleRequestWithKeys(c.Writer, c.Request, ctxMap)
}

func addUser(id string) {
	statsLock.Lock()
	if _, ok := stats[id]; !ok {
		stats[id] = 0
	}
	stats[id] += 1
	statsLock.Unlock()
}

func removeUser(id string, jointime time.Time) {
	// watched at least 5 minutes of the lecture? Count as view.
	if jointime.Before(time.Now().Add(time.Minute * -5)) {
		dao.AddVodView(id)
	}
	statsLock.Lock()
	stats[id] -= 1
	if stats[id] <= 0 {
		delete(stats, id)
	}
	statsLock.Unlock()
}
