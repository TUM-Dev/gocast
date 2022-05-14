package api

import (
	"errors"
	"fmt"
	go_anel_pwrctrl "github.com/RBG-TUM/go-anel-pwrctrl"
	goextron "github.com/RBG-TUM/go-extron"
	"github.com/getsentry/sentry-go"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/joschahenningsen/TUM-Live/dao"
	"github.com/joschahenningsen/TUM-Live/model"
	"github.com/joschahenningsen/TUM-Live/tools"
	"github.com/joschahenningsen/TUM-Live/tools/bot"
	log "github.com/sirupsen/logrus"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	MAX_FILE_SIZE = 1000 * 1000 * 50 // 50 MB
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
	adminG.GET("/api/stream/:streamID/issue", routes.reportStreamIssue)
	adminG.POST("/api/stream/:streamID/sections", routes.createVideoSectionBatch)
	adminG.DELETE("/api/stream/:streamID/sections/:id", routes.deleteVideoSection)

	// group for non-admin web api
	g := router.Group("/")
	g.Use(tools.InitStream(daoWrapper))
	g.GET("/api/stream/:streamID/sections", routes.getVideoSections)

	adminG.POST("/api/stream/:streamID/files", newAttachment)
	adminG.DELETE("/api/stream/:streamID/files/:fid", deleteAttachment)
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
	// Check if stream starts today
	if stream.Start.Truncate(time.Hour*24) != time.Now().Truncate(time.Hour*24) {
		sentry.CaptureException(errors.New("tried to send report for stream that is not active today"))
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	lectureHall, err := r.LectureHallsDao.GetLectureHallByID(stream.LectureHallID)
	if err != nil {
		sentry.CaptureException(err)
		c.AbortWithStatus(http.StatusInternalServerError)
	}
	course, err := r.CoursesDao.GetCourseById(c, stream.CourseID)
	if err != nil {
		sentry.CaptureException(err)
		c.AbortWithStatus(http.StatusInternalServerError)
	}
	streamUrl := tools.Cfg.WebUrl + "/w/" + course.Slug + "/" + fmt.Sprintf("%d", stream.ID)
	botInfo := bot.InfoMessage{
		CourseName:  course.Name,
		LectureHall: lectureHall.Name,
		StreamUrl:   streamUrl,
		CombIP:      lectureHall.CombIP,
		CameraIP:    lectureHall.CameraIP,
	}
	// Set messaging strategy as specified in strategy pattern
	botInfo.SetMessagingMethod(&bot.Matrix{})
	err = botInfo.BotUpdate(botInfo)

	if err != nil {
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
	c.JSON(http.StatusOK, sections)
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

func newAttachment(c *gin.Context) {
	foundContext, _ := c.Get("TUMLiveContext")
	tumLiveContext := foundContext.(tools.TUMLiveContext)
	stream := *tumLiveContext.Stream
	course := *tumLiveContext.Course

	var path string
	var filename string

	switch c.Query("type") {
	case "file":
		file, err := c.FormFile("file")
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, "missing form parameter 'file'")
		}

		if file.Size > MAX_FILE_SIZE {
			c.AbortWithStatusJSON(http.StatusBadRequest, "file too large (limit is 50mb)")
		}

		filename = file.Filename
		fileUuid, err := uuid.NewUUID()
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, "error creating new uuid")
		}

		filesFolder := fmt.Sprintf("%s/%s.%d/%s.%s/files",
			tools.Cfg.Paths.Mass,
			course.Name, course.Year,
			course.Name, course.TeachingTerm)
		path = fmt.Sprintf("%s/%s%s", filesFolder, fileUuid, filepath.Ext(file.Filename))

		err = os.MkdirAll(filesFolder, os.ModePerm)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, "couldn't create folder: "+filesFolder)
		}

		if err = c.SaveUploadedFile(file, path); err != nil {
			log.WithError(err).Error("could not save file with path: " + path)
			c.AbortWithStatusJSON(http.StatusInternalServerError, "could not save file with path: "+path)
		}
	case "url":
		path = c.PostForm("file_url")
		_, filename = filepath.Split(path)
		if path == "" {
			c.AbortWithStatusJSON(http.StatusBadRequest, "missing form parameter 'file_url'")
			return
		}
	default:
		c.AbortWithStatusJSON(http.StatusBadRequest, "missing query parameter 'type'")
		return
	}

	file := model.File{StreamID: stream.ID, Path: path, Filename: filename, Type: model.FILETYPE_ATTACHMENT}
	if dao.File.NewFile(&file) != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, "could not save file in database")
	}

	c.JSON(http.StatusOK, file.ID)
}

func deleteAttachment(c *gin.Context) {
	toDelete, err := dao.File.GetFileById(c.Param("fid"))
	if err != nil {
		c.AbortWithStatus(http.StatusBadRequest)
	}
	if !toDelete.IsAbsolutePath() {
		err = os.Remove(toDelete.Path)
		if err != nil {
			log.WithError(err).Error("could not delete file with path: " + toDelete.Path)
			c.AbortWithStatus(http.StatusInternalServerError)
		}
	}
	err = dao.File.DeleteFile(toDelete.ID)
	if err != nil {
		log.WithError(err).Error("could not delete file from database")
		c.AbortWithStatus(http.StatusInternalServerError)
	}
}
