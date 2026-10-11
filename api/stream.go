package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"google.golang.org/grpc/metadata"

	"github.com/gin-gonic/gin"
	uuid "github.com/satori/go.uuid"
	"gorm.io/gorm"

	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/model"
	"github.com/TUM-Dev/gocast/pkg/runner_manager"
	"github.com/TUM-Dev/gocast/tools"
	"github.com/TUM-Dev/gocast/tools/safepath"
	"github.com/TUM-Dev/gocast/voice-service/pb"
)

const (
	MAX_FILE_SIZE = 1000 * 1000 * 50 // 50 MB
)

func configGinStreamRestRouter(router *gin.Engine, daoWrapper dao.DaoWrapper, manager *runner_manager.Manager) {
	routes := streamRoutes{daoWrapper, manager}

	stream := router.Group("/api/stream")
	{
		// Endpoint for API users with token
		stream.GET("/live", tools.AdminToken(daoWrapper), routes.liveStreams)

		streamById := stream.Group("/:streamID")
		streamById.Use(tools.InitStream(daoWrapper))
		{
			// Admin-Only Endpoints
			admins := streamById.Group("")
			admins.Use(tools.AdminOfCourse)
			admins.GET("", routes.getStream)
			admins.GET("/thumb", routes.RegenerateThumbs)
			admins.PATCH("/visibility", routes.updateStreamVisibility)
			admins.PATCH("/chat/enabled", routes.updateChatEnabled)
			admins.POST("/", routes.putCustomLiveThumbnail)
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
	manager *runner_manager.Manager
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
		logger.Error("Error getting current live", "err", err)
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
			logger.Error("Error fetching course", "err", err)
		}
		lectureHall := "Selfstream"
		if s.LectureHallID != 0 {
			l, err := r.LectureHallsDao.GetLectureHallByID(s.LectureHallID)
			if err != nil {
				logger.Error("Error fetching lecture hall", "err", err)
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

func (r streamRoutes) getStream(c *gin.Context) {
	tumLiveContext := c.MustGet("TUMLiveContext").(tools.TUMLiveContext)

	stream := *tumLiveContext.Stream
	course := *tumLiveContext.Course
	user := tumLiveContext.User
	if stream.Private && (user == nil || !user.IsAdminOfCourse(course)) {
		return
	}

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
		"vod":         stream.Recording,
	})
}

// RegenerateThumbs regenerates the thumbnails for a stream.
func (r streamRoutes) RegenerateThumbs(c *gin.Context) {
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
				logger.Error(fmt.Sprintf("Can't regenerate thumbnail for stream %d with file %s", stream.ID, file.Path))
				continue
			}
			sections, err := r.DaoWrapper.VideoSectionDao.GetByStreamId(stream.ID)
			if err != nil {
				logger.Error(fmt.Sprintf("Can't get video sections for stream %d", stream.ID))
				continue
			}
			err = tools.SetSignedPlaylists(stream, nil, false)
			if err != nil {
				logger.Error(fmt.Sprintf("Can't set signed playlists for stream %d", stream.ID))
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
				err := runner_manager.GenerateSectionImages(r.manager,
					runner_manager.SectionImageRequest{StreamID: stream.ID, PlaylistURL: stream.PlaylistUrl, Sections: sections},
					func() error { return GenerateVideoSectionImages(r.DaoWrapper, &parameters) })
				if err != nil {
					logger.Error("failed to generate video section images", "err", err)
				}
			}()
		}
	}
}

func (r streamRoutes) createVideoSectionBatch(c *gin.Context) {
	context := c.MustGet("TUMLiveContext").(tools.TUMLiveContext)
	stream := context.Stream
	var sections []model.VideoSection
	if err := c.BindJSON(&sections); err != nil {
		logger.Error("failed to bind video section JSON", "err", err)
		_ = c.Error(tools.RequestError{
			Status:        http.StatusBadRequest,
			CustomMessage: "can not bind body",
			Err:           err,
		})
		return
	}

	err := r.VideoSectionDao.Create(sections)
	if err != nil {
		logger.Error("failed to create video sections", "err", err)
		_ = c.Error(tools.RequestError{
			Status:        http.StatusInternalServerError,
			CustomMessage: "failed to create video sections",
			Err:           err,
		})
		return
	}

	sections, err = r.VideoSectionDao.GetByStreamId(context.Stream.ID)
	if err != nil {
		logger.Error("failed to get video sections", "err", err)
		_ = c.Error(tools.RequestError{
			Status:        http.StatusInternalServerError,
			CustomMessage: "failed to get video sections",
			Err:           err,
		})
		return
	}

	err = tools.SetSignedPlaylists(stream, nil, false)
	if err != nil {
		logger.Error("failed to set signed playlists", "err", err)
		_ = c.Error(tools.RequestError{
			Status:        http.StatusInternalServerError,
			CustomMessage: "failed to set signed playlists",
			Err:           err,
		})
		return
	}
	go func() {
		parameters := generateVideoSectionImagesParameters{
			sections:           sections,
			playlistUrl:        stream.PlaylistUrl,
			courseName:         context.Course.Name,
			courseTeachingTerm: context.Course.TeachingTerm,
			courseYear:         uint32(context.Course.Year),
		}
		err := runner_manager.GenerateSectionImages(r.manager,
			runner_manager.SectionImageRequest{StreamID: stream.ID, PlaylistURL: stream.PlaylistUrl, Sections: sections},
			func() error { return GenerateVideoSectionImages(r.DaoWrapper, &parameters) })
		if err != nil {
			logger.Error("failed to generate video section images", "err", err)
		}
	}()

	c.JSON(http.StatusOK, sections)
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
		logger.Error("can not parse video-section id in request url", "err", err)
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
		logger.Error("failed to bind video section JSON", "err", err)
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
		StartSeconds: update.StartSeconds,
	})
	if err != nil {
		logger.Error("failed to update video section", "err", err)
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
		logger.Error("can not parse video-section id in request url", "err", err)
		_ = c.Error(tools.RequestError{
			Status:        http.StatusBadRequest,
			CustomMessage: "can not parse video-section id in request url",
			Err:           err,
		})
		return
	}

	old, err := r.VideoSectionDao.Get(uint(id))
	if err != nil {
		logger.Error("invalid video-section id", "err", err)
		_ = c.Error(tools.RequestError{
			Status:        http.StatusBadRequest,
			CustomMessage: "invalid video-section id",
			Err:           err,
		})
		return
	}

	err = r.VideoSectionDao.Delete(uint(id))
	if err != nil {
		logger.Error("can not delete video-section", "err", err)
		_ = c.Error(tools.RequestError{
			Status:        http.StatusInternalServerError,
			CustomMessage: "can not delete video-section",
			Err:           err,
		})
		return
	}

	file, err := r.FileDao.GetFileById(fmt.Sprintf("%d", old.FileID))
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		logger.Error("can not get video section thumbnail file", "err", err)
		_ = c.Error(tools.RequestError{
			Status:        http.StatusInternalServerError,
			CustomMessage: "can not get video section thumbnail file",
			Err:           err,
		})
		return
	}
	go func() {
		err := DeleteVideoSectionImage(r.DaoWrapper.WorkerDao, file.Path)
		if err != nil {
			logger.Error("failed to generate video section images", "err", err)
		}
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

		// The course name is free text, so it is sanitized before it becomes a
		// directory name - otherwise a name containing path elements would
		// place the upload outside of the mass storage directory.
		filesFolder, err := safepath.JoinInRoot(tools.Cfg.Paths.Mass,
			fmt.Sprintf("%s.%d", course.Name, course.Year),
			fmt.Sprintf("%s.%s", course.Name, course.TeachingTerm),
			"files")
		if err != nil {
			logger.Error("could not build attachment folder", "err", err)
			_ = c.Error(tools.RequestError{
				Status:        http.StatusInternalServerError,
				CustomMessage: "couldn't create folder for attachment",
				Err:           err,
			})
			return
		}
		path = filepath.Join(filesFolder, fmt.Sprintf("%s%s", fileUuid, filepath.Ext(file.Filename)))

		err = os.MkdirAll(filesFolder, 0o755)
		if err != nil {
			_ = c.Error(tools.RequestError{
				Status:        http.StatusInternalServerError,
				CustomMessage: "couldn't create folder: " + filesFolder,
				Err:           err,
			})
			return
		}

		if err = c.SaveUploadedFile(file, path); err != nil {
			logger.Error("could not save file with path: "+path, "err", err)
			_ = c.Error(tools.RequestError{
				Status:        http.StatusInternalServerError,
				CustomMessage: "could not save file with path: " + path,
				Err:           err,
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
			logger.Error("can not delete file with path: "+toDelete.Path, "err", err)
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
		logger.Error("can not delete file from database", "err", err)
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
		logger.Error("can not bind body", "err", err)
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

	var playlist string
	switch {
	case stream.PlaylistUrl != "":
		playlist = stream.PlaylistUrl
	case stream.PlaylistUrlPRES != "":
		playlist = stream.PlaylistUrlPRES
	case stream.PlaylistUrlCAM != "":
		playlist = stream.PlaylistUrlCAM
	default:
		_ = c.Error(tools.RequestError{
			Status:        http.StatusBadRequest,
			CustomMessage: "no playlist found",
		})
		return
	}

	// request to voice-service for subtitles
	client, err := GetSubtitleGeneratorClient()
	if err != nil {
		logger.Error("could not connect to voice-service", "err", err)
		_ = c.Error(tools.RequestError{
			Status:        http.StatusInternalServerError,
			CustomMessage: "could not connect to voice-service",
			Err:           err,
		})
		return
	}
	defer client.CloseConn()

	outCtx := context.Background()
	if tools.Cfg.VoiceService.AuthToken != "" {
		outCtx = metadata.AppendToOutgoingContext(outCtx, "auth", tools.Cfg.VoiceService.AuthToken)
	}
	_, err = client.Generate(outCtx, &pb.GenerateRequest{
		StreamId:   int32(stream.ID),
		SourceFile: playlist,
		Language:   request.Language,
	})
	if err != nil {
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
		logger.Error("Create Audit", "err", err)
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

func (r streamRoutes) putCustomLiveThumbnail(c *gin.Context) {
	tumLiveContext := c.MustGet("TUMLiveContext").(tools.TUMLiveContext)
	streamID := tumLiveContext.Stream.ID
	course := tumLiveContext.Course
	file, err := c.FormFile("file")
	if err != nil {
		_ = c.AbortWithError(http.StatusBadRequest, tools.RequestError{
			Status:        http.StatusBadRequest,
			CustomMessage: "Invalid file",
			Err:           err,
		})
		return
	}

	filename := file.Filename
	fileUuid := uuid.NewV1()

	// Enforce reasonable file size limit for thumbnails.
	if file.Size > MAX_FILE_SIZE {
		_ = c.AbortWithError(http.StatusBadRequest, tools.RequestError{
			Status:        http.StatusBadRequest,
			CustomMessage: "File too large",
			Err:           errors.New("uploaded file exceeds maximum allowed size"),
		})
		return
	}
	// Validate that the uploaded file is an image (MIME type and file extension).
	contentType := file.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "image/") {
		_ = c.AbortWithError(http.StatusBadRequest, tools.RequestError{
			Status:        http.StatusBadRequest,
			CustomMessage: "Invalid file type",
			Err:           errors.New("uploaded file is not an image"),
		})
		return
	}
	ext := strings.ToLower(filepath.Ext(file.Filename))
	switch ext {
	case ".jpg", ".jpeg", ".png", ".gif", ".webp":
	default:
		_ = c.AbortWithError(http.StatusBadRequest, tools.RequestError{
			Status:        http.StatusBadRequest,
			CustomMessage: "Unsupported image format",
			Err:           errors.New("unsupported image file extension"),
		})
		return
	}

	// See newAttachment: the course name is free text and must not be able to
	// steer the thumbnail out of the mass storage directory.
	filesFolder, err := safepath.JoinInRoot(
		tools.Cfg.Paths.Mass,
		fmt.Sprintf("%s.%d.%s", course.Name, course.Year, course.TeachingTerm),
		"files")
	if err != nil {
		logger.Error("could not build thumbnail folder", "err", err)
		_ = c.AbortWithError(http.StatusInternalServerError, tools.RequestError{
			Status:        http.StatusInternalServerError,
			CustomMessage: "Failed to create thumbnail folder",
			Err:           err,
		})
		return
	}

	if err := os.MkdirAll(filesFolder, 0o755); err != nil {
		_ = c.AbortWithError(http.StatusInternalServerError, tools.RequestError{
			Status:        http.StatusInternalServerError,
			CustomMessage: "Failed to create thumbnail folder",
			Err:           err,
		})
		return
	}

	path := filepath.Join(
		filesFolder,
		fmt.Sprintf("%s%s", fileUuid, filepath.Ext(filename)))

	if err := c.SaveUploadedFile(file, path); err != nil {
		_ = c.AbortWithError(http.StatusInternalServerError, tools.RequestError{
			Status:        http.StatusInternalServerError,
			CustomMessage: "Failed to save file",
			Err:           err,
		})
		return
	}

	thumb := model.File{
		StreamID:   streamID,
		Path:       path,
		Filename:   file.Filename,
		Type:       model.FILETYPE_THUMB_CUSTOM,
		CourseName: course.Name,
	}

	if err := r.DaoWrapper.FileDao.SetThumbnail(streamID, thumb); err != nil {
		_ = c.AbortWithError(http.StatusInternalServerError, tools.RequestError{
			Status:        http.StatusInternalServerError,
			CustomMessage: "Failed to set thumbnail",
			Err:           err,
		})
		return
	}

	c.JSON(http.StatusOK, thumb.ID)
}
