package api

import (
	"context"
	"errors"
	"fmt"
	"github.com/joschahenningsen/TUM-Live/tools/pathprovider"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/gin-gonic/gin"
	"github.com/joschahenningsen/TUM-Live/dao"
	"github.com/joschahenningsen/TUM-Live/model"
	"github.com/joschahenningsen/TUM-Live/tools"
	"github.com/joschahenningsen/TUM-Live/tools/bot"
	"github.com/joschahenningsen/TUM-Live/voice-service/pb"
	uuid "github.com/satori/go.uuid"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

const (
	MAX_FILE_SIZE = 1000 * 1000 * 50 // 50 MB
)

func configGinStreamRestRouter(router *gin.Engine, daoWrapper dao.DaoWrapper) {
	routes := streamRoutes{daoWrapper}

	stream := router.Group("/api/stream")
	{
		// Endpoint for API users with token
		stream.GET("/live", tools.AdminToken(daoWrapper), routes.liveStreams)

		streamById := stream.Group("/:streamID")
		streamById.Use(tools.InitStream(daoWrapper))
		{
			// All User Endpoints
			streamById.GET("/sections", routes.getVideoSections)
			streamById.GET("/subtitles/:lang", routes.getSubtitles)

			thumbs := streamById.Group("/thumbs")
			{
				thumbs.GET(":fid", routes.getThumbs)
				thumbs.GET("/live", routes.getLiveThumbs)
				thumbs.GET("/vod", routes.getVODThumbs)
			}
		}
		{
			// Admin-Only Endpoints
			admins := streamById.Group("")
			admins.Use(tools.AdminOfCourse)
			admins.GET("", routes.getStream)
			admins.GET("/end", routes.endStream)
			admins.GET("/thumb", routes.RegenerateThumbs)
			admins.POST("/issue", routes.reportStreamIssue)
			admins.PATCH("/visibility", routes.updateStreamVisibility)
			admins.PATCH("/chat/enabled", routes.updateChatEnabled)
			sections := admins.Group("/sections")
			{
				sections.POST("", routes.createVideoSectionBatch)
				sections.PUT("/:id", routes.updateVideoSection)
				sections.DELETE("/:id", routes.deleteVideoSection)
			}

			files := admins.Group("files")
			{
				files.POST("", routes.newAttachment)
				files.DELETE("/:fid", routes.deleteAttachment)
			}

			subtitles := admins.Group("subtitles")
			{
				subtitles.POST("", routes.requestSubtitles)
			}
		}
	}
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

func (r streamRoutes) getThumbs(c *gin.Context) {
	ctx, exists := c.Get("TUMLiveContext")
	tumLiveContext := ctx.(tools.TUMLiveContext)

	if !exists {
		sentry.CaptureException(errors.New("context should exist but doesn't"))
		_ = c.Error(tools.RequestError{
			Status:        http.StatusInternalServerError,
			CustomMessage: "context should exist but doesn't",
		})
		return
	}
	file, err := r.GetFileById(c.Param("fid"))
	if err != nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusNotFound,
			CustomMessage: "can not find file",
			Err:           err,
		})
		return
	}
	if !file.IsThumb() {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusForbidden,
			CustomMessage: "file is not a thumbnail",
		})
		return
	}
	if tumLiveContext.Stream.ID != file.StreamID {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusBadRequest,
			CustomMessage: "streamID of file doesn't match stream id of request url",
		})
		return
	}
	sendDownloadFile(c, file, tumLiveContext)
}

func (r streamRoutes) getVODThumbs(c *gin.Context) {
	tumLiveContext := c.MustGet("TUMLiveContext").(tools.TUMLiveContext)

	thumb, err := tumLiveContext.Stream.GetLGThumbnail()
	if err != nil {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	c.File(thumb)
}

func (r streamRoutes) getLiveThumbs(c *gin.Context) {
	tumLiveContext := c.MustGet("TUMLiveContext").(tools.TUMLiveContext)

	streamId := strconv.Itoa(int(tumLiveContext.Stream.ID))
	path := pathprovider.LiveThumbnail(streamId)
	c.File(path)
}

func (r streamRoutes) getSubtitles(c *gin.Context) {
	ctx := c.MustGet("TUMLiveContext").(tools.TUMLiveContext)
	lang := c.Param("lang")

	subtitlesObj, err := r.SubtitlesDao.GetByStreamIDandLang(context.Background(), ctx.Stream.ID, lang)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			_ = c.Error(tools.RequestError{
				Status:        http.StatusNotFound,
				CustomMessage: "invalid streamID or language",
			})
		} else {
			_ = c.Error(tools.RequestError{
				Err:           err,
				Status:        http.StatusInternalServerError,
				CustomMessage: "can not get by streamID and language",
			})
		}
		return
	}
	c.Data(http.StatusOK, "text/vtt", []byte(subtitlesObj.Content))
}

// livestreams returns all streams that are live
func (r streamRoutes) liveStreams(c *gin.Context) {
	var res []liveStreamDto
	streams, err := r.StreamsDao.GetCurrentLive(c)
	if err != nil {
		log.Error(err)
		_ = c.Error(tools.RequestError{
			Status:        http.StatusInternalServerError,
			CustomMessage: "can not get current live streams",
			Err:           err,
		})
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
	/*tumLiveContext := c.MustGet("TUMLiveContext").(tools.TUMLiveContext)
	discardVoD := c.Request.URL.Query().Get("discard") == "true"
	log.Info(discardVoD)*/
	// todo NotifyWorkersToStopStream(*tumLiveContext.Stream, discardVoD, r.DaoWrapper)
}

// reportStreamIssue sends a notification to a matrix room that can be used for debugging technical issues.
func (r streamRoutes) reportStreamIssue(c *gin.Context) {
	tumLiveContext := c.MustGet("TUMLiveContext").(tools.TUMLiveContext)

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
		_ = c.Error(tools.RequestError{
			Status:        http.StatusBadRequest,
			CustomMessage: "can not bind body",
			Err:           err,
		})
	}

	// Get lecture hall of the stream that has issues.
	lectureHall, err := r.LectureHallsDao.GetLectureHallByID(stream.LectureHallID)
	if err != nil {
		sentry.CaptureException(err)
		_ = c.Error(tools.RequestError{
			Status:        http.StatusInternalServerError,
			CustomMessage: "can not get lecturehall by id",
			Err:           err,
		})
		return
	}

	// Get course of the stream that has issues.
	course, err := r.CoursesDao.GetCourseById(c, stream.CourseID)
	if err != nil {
		sentry.CaptureException(err)
		_ = c.Error(tools.RequestError{
			Status:        http.StatusInternalServerError,
			CustomMessage: "can not get course by id",
			Err:           err,
		})
		return
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
		_ = c.Error(tools.RequestError{
			Status:        http.StatusInternalServerError,
			CustomMessage: "can not send bot alert",
			Err:           err,
		})
	}
}

func (r streamRoutes) getStream(c *gin.Context) {
	tumLiveContext := c.MustGet("TUMLiveContext").(tools.TUMLiveContext)

	stream := *tumLiveContext.Stream
	course := *tumLiveContext.Course

	c.JSON(http.StatusOK, gin.H{
		"course":      course.Name,
		"courseID":    course.ID,
		"streamID":    stream.ID,
		"name":        stream.Name,
		"description": stream.Description,
		"start":       stream.Start,
		"end":         stream.End,
		"ingest":      fmt.Sprintf("%s%s-%d?secret=%s", tools.Cfg.IngestBase, course.Slug, stream.ID, stream.StreamKey),
		"live":        stream.LiveNow,
		"vod":         stream.Recording})
}

func (r streamRoutes) getVideoSections(c *gin.Context) {
	tumLiveContext := c.MustGet("TUMLiveContext").(tools.TUMLiveContext)
	sections, err := r.VideoSectionDao.GetByStreamId(tumLiveContext.Stream.ID)
	if err != nil {
		log.WithError(err).Error("Can't get video sections")
	}

	c.JSON(http.StatusOK, sections)
}

// RegenerateThumbs regenerates the thumbnails for a stream.
func (r streamRoutes) RegenerateThumbs(c *gin.Context) {
	/* todo: is this required?
	tumLiveContext := c.MustGet("TUMLiveContext").(tools.TUMLiveContext)
	stream := tumLiveContext.Stream
	course := tumLiveContext.Course
	_ = stream
	for _, file := range stream.Files {
		if file.Type == model.FILETYPE_VOD {
			// Unlike for generating video sections, we need a new method here, as there is no API in place.
			// The thumbnails are generated automatically by the worker which then notifies the backend.
			err := RegenerateThumbs(r.DaoWrapper, file, stream, course)
			if err != nil {
				log.WithError(err).Errorf("Can't regenerate thumbnail for stream %d with file %s", stream.ID, file.Path)
				continue
			}
			sections, err := r.DaoWrapper.VideoSectionDao.GetByStreamId(stream.ID)
			if err != nil {
				log.WithError(err).Errorf("Can't get video sections for stream %d", stream.ID)
				continue
			}
			err = tools.SetSignedPlaylists(stream, nil, false)
			if err != nil {
				log.WithError(err).Errorf("Can't set signed playlists for stream %d", stream.ID)
				continue
			}
			// Completely redo the video section image generation. This also updates the database, if the naming scheme has changed.
			go func() {
				parameters := generateVideoSectionImagesParameters{
					sections:           sections,
					playlistUrl:        stream.PlaylistUrl,
					courseName:         course.Name,
					courseTeachingTerm: course.TeachingTerm,
					courseYear:         uint32(tumLiveContext.Course.Year),
				}
				err := GenerateVideoSectionImages(r.DaoWrapper, &parameters)
				if err != nil {
					log.WithError(err).Error("failed to generate video section images")
				}
			}()
		}
	}*/
}

func (r streamRoutes) createVideoSectionBatch(c *gin.Context) {
	context := c.MustGet("TUMLiveContext").(tools.TUMLiveContext)
	stream := context.Stream
	var sections []model.VideoSection
	if err := c.BindJSON(&sections); err != nil {
		log.WithError(err).Error("failed to bind video section JSON")
		_ = c.Error(tools.RequestError{
			Status:        http.StatusBadRequest,
			CustomMessage: "can not bind body",
			Err:           err,
		})
		return
	}

	err := r.VideoSectionDao.Create(sections)
	if err != nil {
		log.WithError(err).Error("failed to create video sections")
		_ = c.Error(tools.RequestError{
			Status:        http.StatusInternalServerError,
			CustomMessage: "failed to create video sections",
			Err:           err,
		})
		return
	}

	sections, err = r.VideoSectionDao.GetByStreamId(context.Stream.ID)
	if err != nil {
		log.WithError(err).Error("failed to get video sections")
		_ = c.Error(tools.RequestError{
			Status:        http.StatusInternalServerError,
			CustomMessage: "failed to get video sections",
			Err:           err,
		})
		return
	}

	err = tools.SetSignedPlaylists(stream, nil, false)
	if err != nil {
		log.WithError(err).Error("failed to set signed playlists")
		_ = c.Error(tools.RequestError{
			Status:        http.StatusInternalServerError,
			CustomMessage: "failed to set signed playlists",
			Err:           err,
		})
		return
	}
	go func() {
		/* todo
		parameters := generateVideoSectionImagesParameters{
			sections:           sections,
			playlistUrl:        stream.PlaylistUrl,
			courseName:         context.Course.Name,
			courseTeachingTerm: context.Course.TeachingTerm,
			courseYear:         uint32(context.Course.Year),
		}
		err := GenerateVideoSectionImages(r.DaoWrapper, &parameters)
		if err != nil {
			log.WithError(err).Error("failed to generate video section images")
		}
		*/
	}()
}

type UpdateVideoSectionRequest struct {
	Description  string `json:"description"`
	StartHours   uint   `json:"startHours"`
	StartMinutes uint   `json:"startMinutes"`
	StartSeconds uint   `json:"startSeconds"`
}

func (r streamRoutes) updateVideoSection(c *gin.Context) {
	idAsString := c.Param("id")
	id, err := strconv.Atoi(idAsString)
	if err != nil {
		log.WithError(err).Error("can not parse video-section id in request url")
		_ = c.Error(tools.RequestError{
			Status:        http.StatusBadRequest,
			CustomMessage: "can not parse video-section id in request url",
			Err:           err,
		})
		return
	}

	var update UpdateVideoSectionRequest
	err = c.BindJSON(&update)
	if err != nil {
		log.WithError(err).Error("failed to bind video section JSON")
		_ = c.Error(tools.RequestError{
			Status:        http.StatusBadRequest,
			CustomMessage: "can not bind body",
			Err:           err,
		})
		return
	}

	err = r.VideoSectionDao.Update(&model.VideoSection{
		Model:        gorm.Model{ID: uint(id)},
		Description:  update.Description,
		StartHours:   update.StartHours,
		StartMinutes: update.StartMinutes,
		StartSeconds: update.StartSeconds})
	if err != nil {
		log.WithError(err).Error("failed to update video section")
		_ = c.Error(tools.RequestError{
			Status:        http.StatusInternalServerError,
			CustomMessage: "can not update video section",
			Err:           err,
		})
		return
	}
}

func (r streamRoutes) deleteVideoSection(c *gin.Context) {
	idAsString := c.Param("id")
	id, err := strconv.Atoi(idAsString)
	if err != nil {
		log.WithError(err).Error("can not parse video-section id in request url")
		_ = c.Error(tools.RequestError{
			Status:        http.StatusBadRequest,
			CustomMessage: "can not parse video-section id in request url",
			Err:           err,
		})
		return
	}

	old, err := r.VideoSectionDao.Get(uint(id))
	if err != nil {
		log.WithError(err).Error("invalid video-section id")
		_ = c.Error(tools.RequestError{
			Status:        http.StatusBadRequest,
			CustomMessage: "invalid video-section id",
			Err:           err,
		})
		return
	}

	_, err = r.FileDao.GetFileById(fmt.Sprintf("%d", old.FileID))
	if err != nil {
		log.WithError(err).Error("can not find file")
		_ = c.Error(tools.RequestError{
			Status:        http.StatusNotFound,
			CustomMessage: "can not find file",
			Err:           err,
		})
		return
	}

	err = r.VideoSectionDao.Delete(uint(id))
	if err != nil {
		log.WithError(err).Error("can not delete video-section")
		_ = c.Error(tools.RequestError{
			Status:        http.StatusInternalServerError,
			CustomMessage: "can not delete video-section",
			Err:           err,
		})
		return
	}

	go func() {
		/* todo
		err := DeleteVideoSectionImage(r.DaoWrapper.WorkerDao, file.Path)
		if err != nil {
			log.WithError(err).Error("failed to generate video section images")
		}
		*/
	}()

	c.Status(http.StatusAccepted)
}

func (r streamRoutes) newAttachment(c *gin.Context) {
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
			_ = c.Error(tools.RequestError{
				Status:        http.StatusBadRequest,
				CustomMessage: "missing form parameter 'file'",
				Err:           err,
			})
			return
		}

		if file.Size > MAX_FILE_SIZE {
			_ = c.Error(tools.RequestError{
				Status:        http.StatusBadRequest,
				CustomMessage: "file too large (limit is 50mb)",
			})
			return
		}

		filename = file.Filename
		fileUuid := uuid.NewV1()

		filesFolder := fmt.Sprintf("%s/%s.%d/%s.%s/files",
			tools.Cfg.Paths.Mass,
			course.Name, course.Year,
			course.Name, course.TeachingTerm)
		path = fmt.Sprintf("%s/%s%s", filesFolder, fileUuid, filepath.Ext(file.Filename))

		err = os.MkdirAll(filesFolder, os.ModePerm)
		if err != nil {
			_ = c.Error(tools.RequestError{
				Status:        http.StatusInternalServerError,
				CustomMessage: "couldn't create folder: " + filesFolder,
				Err:           err,
			})
			return
		}

		if err = c.SaveUploadedFile(file, path); err != nil {
			log.WithError(err).Error("could not save file with path: " + path)
			_ = c.Error(tools.RequestError{
				Status:        http.StatusInternalServerError,
				CustomMessage: "could not save file with path: " + path,
				Err:           err,
			})
			return
		}
	case "url":
		path = c.PostForm("file_url")
		_, filename = filepath.Split(path)
		if path == "" {
			_ = c.Error(tools.RequestError{
				Status:        http.StatusBadRequest,
				CustomMessage: "missing form parameter 'file_url'",
			})
			return
		}
	default:
		_ = c.Error(tools.RequestError{
			Status:        http.StatusBadRequest,
			CustomMessage: "missing or invalid query parameter 'type'",
		})
		return
	}

	file := model.File{StreamID: stream.ID, Path: path, Filename: filename, Type: model.FILETYPE_ATTACHMENT}
	if err := r.FileDao.NewFile(&file); err != nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusInternalServerError,
			CustomMessage: "can not save file in database",
			Err:           err,
		})
		return
	}

	c.JSON(http.StatusOK, file.ID)
}

func (r streamRoutes) deleteAttachment(c *gin.Context) {
	toDelete, err := r.FileDao.GetFileById(c.Param("fid"))
	if err != nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusBadRequest,
			CustomMessage: "can not find file",
			Err:           err,
		})
		return
	}
	if !toDelete.IsURL() {
		err = os.Remove(toDelete.Path)
		if err != nil {
			log.WithError(err).Error("can not delete file with path: " + toDelete.Path)
			_ = c.Error(tools.RequestError{
				Status:        http.StatusInternalServerError,
				CustomMessage: "can not delete file with path: " + toDelete.Path,
				Err:           err,
			})
			return
		}
	}
	err = r.FileDao.DeleteFile(toDelete.ID)
	if err != nil {
		log.WithError(err).Error("can not delete file from database")
		_ = c.Error(tools.RequestError{
			Status:        http.StatusInternalServerError,
			CustomMessage: "can not delete file from database",
			Err:           err,
		})
		return
	}
}

func (r streamRoutes) requestSubtitles(c *gin.Context) {
	tumLiveContext := c.MustGet("TUMLiveContext").(tools.TUMLiveContext)

	stream := tumLiveContext.Stream

	type subtitleRequest struct {
		Language string `json:"language"`
	}

	var request subtitleRequest
	err := c.BindJSON(&request)
	if err != nil {
		sentry.CaptureException(err)
		_ = c.Error(tools.RequestError{
			Status:        http.StatusBadRequest,
			CustomMessage: "can not bind body",
			Err:           err,
		})
		return
	}

	err = tools.SetSignedPlaylists(stream, tumLiveContext.User, false)
	if err != nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusInternalServerError,
			CustomMessage: "can not create signed stream playlists",
			Err:           err,
		})
		return
	}

	playlist := ""
	if stream.PlaylistUrl != "" {
		playlist = stream.PlaylistUrl
	} else if stream.PlaylistUrlCAM != "" {
		playlist = stream.PlaylistUrlCAM
	} else if stream.PlaylistUrlPRES != "" {
		playlist = stream.PlaylistUrlPRES
	} else {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusBadRequest,
			CustomMessage: "no playlist found",
		})
		return
	}

	// request to voice-service for subtitles
	client, err := GetSubtitleGeneratorClient()
	if err != nil {
		sentry.CaptureException(err)
		_ = c.Error(tools.RequestError{
			Status:        http.StatusInternalServerError,
			CustomMessage: "could not connect to voice-service",
			Err:           err,
		})
		return
	}
	defer client.CloseConn()

	_, err = client.Generate(context.Background(), &pb.GenerateRequest{
		StreamId:   int32(stream.ID),
		SourceFile: playlist,
		Language:   request.Language,
	})
	if err != nil {
		sentry.CaptureException(err)
		_ = c.Error(tools.RequestError{
			Status:        http.StatusInternalServerError,
			CustomMessage: "could not call generate on voice_client",
			Err:           err,
		})
		return
	}

	c.Status(http.StatusCreated)
}

func (r streamRoutes) updateStreamVisibility(c *gin.Context) {
	ctx := c.MustGet("TUMLiveContext").(tools.TUMLiveContext)
	var req struct {
		Private bool `json:"private"`
	}
	err := c.BindJSON(&req)
	if err != nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusBadRequest,
			CustomMessage: "can not bind body",
			Err:           err,
		})
		return
	}

	err = r.AuditDao.Create(&model.Audit{
		User:    ctx.User,
		Message: fmt.Sprintf("%d: (Visibility: %v)", ctx.Stream.ID, req.Private), // e.g. "eidi:'Einführung in die Informatik' (2020, S)"
		Type:    model.AuditStreamEdit,
	})
	if err != nil {
		log.Error("Create Audit:", err)
	}

	err = r.DaoWrapper.StreamsDao.ToggleVisibility(ctx.Stream.ID, req.Private)
	if err != nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusInternalServerError,
			CustomMessage: "can not update stream",
			Err:           err,
		})
		return
	}
}

func (r streamRoutes) updateChatEnabled(c *gin.Context) {
	stream, err := r.StreamsDao.GetStreamByID(context.Background(), c.Param("streamID"))
	if err != nil {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}

	var req struct {
		ChatEnabled bool `json:"isChatEnabled"`
	}
	err = c.BindJSON(&req)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, "could not parse request body")
		return
	}

	stream.ChatEnabled = req.ChatEnabled
	err = r.DaoWrapper.StreamsDao.UpdateStream(stream)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, "could not update stream")
		return
	}

}
