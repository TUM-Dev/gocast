package api

import (
	"errors"
	"fmt"
	go_anel_pwrctrl "github.com/RBG-TUM/go-anel-pwrctrl"
	goextron "github.com/RBG-TUM/go-extron"
	"github.com/getsentry/sentry-go"
	"github.com/gin-gonic/gin"
	"github.com/joschahenningsen/TUM-Live/dao"
	"github.com/joschahenningsen/TUM-Live/model"
	"github.com/joschahenningsen/TUM-Live/tools"
	"github.com/joschahenningsen/TUM-Live/tools/bot"
	log "github.com/sirupsen/logrus"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func configGinStreamRestRouter(router *gin.Engine, daoWrapper dao.DaoWrapper) {
	routes := streamRoutes{daoWrapper}
	// group for api users with token
	tokenG := router.Group("/")
	tokenG.Use(tools.AdminToken(daoWrapper))
	tokenG.GET("/api/stream/live", routes.liveStreams)

	// group for web api
	adminG := router.Group("/")
	adminG.Use(tools.InitStream(daoWrapper))
	adminG.Use(tools.AdminOfCourse)
	adminG.GET("/api/stream/:streamID", routes.getStream)
	adminG.GET("/api/stream/:streamID/pause", routes.pauseStream)
	adminG.GET("/api/stream/:streamID/end", routes.endStream)
	adminG.POST("/api/stream/:streamID/issue", routes.reportStreamIssue)
	adminG.POST("/api/stream/:streamID/sections", routes.createVideoSectionBatch)
	adminG.DELETE("/api/stream/:streamID/sections/:id", routes.deleteVideoSection)

	// group for non-admin web api
	g := router.Group("/")
	g.Use(tools.InitStream(daoWrapper))
	g.GET("/api/stream/:streamID/sections", routes.getVideoSections)
}

type streamRoutes struct {
	dao.DaoWrapper
}

type liveStreamDto struct {
	ID          uint
	CourseName  string
	LectureHall string
	COMB        string
	PRES        string
	CAM         string
	End         time.Time
}

// livestreams returns all streams that are live
func (r streamRoutes) liveStreams(c *gin.Context) {
	var res []liveStreamDto
	streams, err := r.StreamsDao.GetCurrentLive(c)
	if err != nil {
		log.Error(err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	for _, s := range streams {
		course, err := r.CoursesDao.GetCourseById(c, s.CourseID)
		if err != nil {
			log.Error(err)
		}
		lectureHall := "Selfstream"
		if s.LectureHallID != 0 {
			l, err := r.LectureHallsDao.GetLectureHallByID(s.LectureHallID)
			if err != nil {
				log.Error(err)
			} else {
				lectureHall = l.Name
			}
		}
		res = append(res, liveStreamDto{
			ID:          s.ID,
			CourseName:  course.Name,
			LectureHall: lectureHall,
			COMB:        s.PlaylistUrl,
			PRES:        s.PlaylistUrlPRES,
			CAM:         s.PlaylistUrlCAM,
			End:         s.End,
		})
	}
	c.JSON(http.StatusOK, res)
}

func (r streamRoutes) endStream(c *gin.Context) {
	foundContext, exists := c.Get("TUMLiveContext")
	if !exists {
		sentry.CaptureException(errors.New("context should exist but doesn't"))
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	discardVoD := c.Request.URL.Query().Get("discard") == "true"
	log.Info(discardVoD)
	tumLiveContext := foundContext.(tools.TUMLiveContext)
	NotifyWorkersToStopStream(*tumLiveContext.Stream, discardVoD, r.DaoWrapper)
}

func (r streamRoutes) pauseStream(c *gin.Context) {
	pause := c.Request.URL.Query().Get("pause") == "true"
	foundContext, exists := c.Get("TUMLiveContext")
	if !exists {
		sentry.CaptureException(errors.New("context should exist but doesn't"))
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	tumLiveContext := foundContext.(tools.TUMLiveContext)
	stream := tumLiveContext.Stream
	lectureHall, err := r.LectureHallsDao.GetLectureHallByID(stream.LectureHallID)
	if err != nil {
		log.WithError(err).Error("request to pause stream without lecture hall")
		return
	}
	ge := goextron.New(fmt.Sprintf("http://%s", strings.ReplaceAll(lectureHall.CombIP, "extron3", "")), tools.Cfg.Auths.SmpUser, tools.Cfg.Auths.SmpUser) // todo
	err = ge.SetMute(pause)
	client := go_anel_pwrctrl.New(lectureHall.PwrCtrlIp, tools.Cfg.Auths.PwrCrtlAuth)
	if pause {
		err := client.TurnOff(lectureHall.LiveLightIndex)
		if err != nil {
			log.WithError(err).Error("can't turn off light")
		}
	} else {
		err := client.TurnOn(lectureHall.LiveLightIndex)
		if err != nil {
			log.WithError(err).Error("can't turn on light")
		}
	}
	if err != nil {
		log.WithError(err).Error("Can't mute/unmute")
		return
	}
	err = r.StreamsDao.SavePauseState(stream.ID, pause)
	if err != nil {
		log.WithError(err).Error("Pause: Can't save stream")
	} else {
		notifyViewersPause(stream.ID, pause)
	}
}

// reportStreamIssue sends a notification to a matrix room that can be used for debugging technical issues.
func (r streamRoutes) reportStreamIssue(c *gin.Context) {
	foundContext, exists := c.Get("TUMLiveContext")
	if !exists {
		sentry.CaptureException(errors.New("context should exist but doesn't"))
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	tumLiveContext := foundContext.(tools.TUMLiveContext)
	stream := tumLiveContext.Stream

	type alertMessage struct {
		Comment     string  `json:"description"`
		PhoneNumber string  `json:"phone"`
		Email       string  `json:"email"`
		Categories  []uint8 `json:"categories"`
		Name        string  `json:"name"`
	}

	var alert alertMessage
	if err := c.ShouldBindJSON(&alert); err != nil {
		sentry.CaptureException(err)
		c.AbortWithStatus(http.StatusBadRequest)
	}

	// Get lecture hall of the stream that has issues.
	lectureHall, err := r.LectureHallsDao.GetLectureHallByID(stream.LectureHallID)
	if err != nil {
		sentry.CaptureException(err)
		c.AbortWithStatus(http.StatusInternalServerError)
	}

	// Get course of the stream that has issues.
	course, err := r.CoursesDao.GetCourseById(c, stream.CourseID)
	if err != nil {
		sentry.CaptureException(err)
		c.AbortWithStatus(http.StatusInternalServerError)
	}

	// Build stream URL, e.g. https://live.rbg.tum.de/w/gbs/1234
	streamUrl := tools.Cfg.WebUrl + "/w/" + course.Slug + "/" + fmt.Sprintf("%d", stream.ID)
	categories := map[uint8]string{1: "🎥 Camera", 2: "🎤 Microphone", 3: "🔊 Audio", 4: "🎬 Video", 5: "💡Light", 6: "Other"}
	var categoryList []string
	for _, category := range alert.Categories {
		categoryList = append(categoryList, categories[category])
	}
	botInfo := bot.AlertMessage{
		PhoneNumber: alert.PhoneNumber,
		Name:        alert.Name,
		Email:       alert.Email,
		Comment:     alert.Comment,
		Categories:  strings.Join(categoryList, " · "),
		CourseName:  course.Name,
		LectureHall: lectureHall.Name,
		StreamUrl:   streamUrl,
		CombIP:      lectureHall.CombIP,
		CameraIP:    lectureHall.CameraIP,
		IsLecturer:  tumLiveContext.User.IsAdminOfCourse(course),
		Stream:      *stream,
		User:        *tumLiveContext.User,
	}

	// Send notification to the matrix room.
	var alertBot bot.Bot
	alertBot.SetMessagingMethod(&bot.Matrix{})

	// Set messaging strategy as specified in strategy pattern
	if err = alertBot.SendAlert(botInfo, r.StatisticsDao); err != nil {
		sentry.CaptureException(err)
		c.AbortWithStatus(http.StatusInternalServerError)
	}
}

func (r streamRoutes) getStream(c *gin.Context) {
	foundContext, exists := c.Get("TUMLiveContext")
	if !exists {
		sentry.CaptureException(errors.New("context should exist but doesn't"))
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	tumLiveContext := foundContext.(tools.TUMLiveContext)

	stream := *tumLiveContext.Stream
	course := *tumLiveContext.Course

	c.JSON(http.StatusOK,
		gin.H{"course": course.Name,
			"courseID":    course.ID,
			"streamID":    stream.ID,
			"name":        stream.Name,
			"description": stream.Description,
			"start":       stream.Start,
			"end":         stream.End,
			"ingest":      fmt.Sprintf("%sstream?secret=%s", tools.Cfg.IngestBase, stream.StreamKey),
			"live":        stream.LiveNow,
			"vod":         stream.Recording})
}

func (r streamRoutes) getVideoSections(c *gin.Context) {
	foundContext, exists := c.Get("TUMLiveContext")
	if !exists {
		sentry.CaptureException(errors.New("context should exist but doesn't"))
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	tumLiveContext := foundContext.(tools.TUMLiveContext)
	sections, err := r.VideoSectionDao.GetByStreamId(tumLiveContext.Stream.ID)
	if err != nil {
		log.WithError(err).Error("Can't get video sections")
	}

	response := []gin.H{}
	for _, section := range sections {
		response = append(response, gin.H{
			"ID":                section.ID,
			"startHours":        section.StartHours,
			"startMinutes":      section.StartMinutes,
			"startSeconds":      section.StartSeconds,
			"description":       section.Description,
			"friendlyTimestamp": section.TimestampAsString(),
			"streamID":          section.StreamID,
		})

	}
	c.JSON(http.StatusOK, response)
}

func (r streamRoutes) createVideoSectionBatch(c *gin.Context) {
	var sections []model.VideoSection
	if err := c.BindJSON(&sections); err != nil {
		log.WithError(err).Error("failed to bind video section JSON")
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	err := r.VideoSectionDao.Create(sections)
	if err != nil {
		log.WithError(err).Error("failed to create video sections")
		c.AbortWithStatus(http.StatusInternalServerError)
	}
}

func (r streamRoutes) deleteVideoSection(c *gin.Context) {
	_, exists := c.Get("TUMLiveContext")
	if !exists {
		sentry.CaptureException(errors.New("context should exist but doesn't"))
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	idAsString := c.Param("id")
	id, err := strconv.Atoi(idAsString)
	if err != nil {
		log.WithError(err).Error("Can't parse video-section id in url")
	}
	err = r.VideoSectionDao.Delete(uint(id))
	if err != nil {
		log.WithError(err).Error("Can't delete video-section")
	}
}
