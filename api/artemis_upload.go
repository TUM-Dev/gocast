package api

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	uuid "github.com/satori/go.uuid"

	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/model"
	"github.com/TUM-Dev/gocast/tools"
)

// configGinArtemisRouter configures the router for Artemis video upload endpoints
func configGinArtemisRouter(router *gin.Engine, daoWrapper dao.DaoWrapper) {
	routes := artemisRoutes{daoWrapper}

	artemis := router.Group("/api/course/artemis/:courseID")
	{
		artemis.POST("/upload", routes.uploadVideo)
	}
}

type artemisRoutes struct {
	dao.DaoWrapper
}

type artemisUploadRequest struct {
	Title       string            `form:"title"`
	Description string            `form:"description"`
	VideoType   model.VideoType   `form:"videoType"`
	StreamStart time.Time         `form:"streamStart"`
	StreamID    uint              `form:"streamID"` // Optional: if provided, upload to existing stream
}

// uploadVideo handles video uploads from Artemis (or any external system) using User API tokens
func (r artemisRoutes) uploadVideo(c *gin.Context) {
	logger.Info("Artemis upload request received")

	// Step 1: Authenticate using User API token
	user, err := r.authenticateByUserToken(c)
	if err != nil {
		logger.Warn("Authentication failed", "err", err)
		_ = c.Error(tools.RequestError{
			Status:        http.StatusUnauthorized,
			CustomMessage: "invalid or expired authentication token",
			Err:           err,
		})
		return
	}

	// Step 2: Get course ID from path parameter
	courseIDStr := c.Param("courseID")
	courseID, err := strconv.ParseUint(courseIDStr, 10, 32)
	if err != nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusBadRequest,
			CustomMessage: "invalid course ID",
			Err:           err,
		})
		return
	}

	// Step 3: Verify course exists
	course, err := r.CoursesDao.GetCourseById(c, uint(courseID))
	if err != nil {
		logger.Warn("Course not found", "courseID", courseID, "err", err)
		_ = c.Error(tools.RequestError{
			Status:        http.StatusNotFound,
			CustomMessage: "course not found",
			Err:           err,
		})
		return
	}

	// Step 4: Verify VOD is enabled for the course
	if !course.VODEnabled {
		logger.Warn("VOD not enabled for course", "courseID", courseID)
		_ = c.Error(tools.RequestError{
			Status:        http.StatusForbidden,
			CustomMessage: "VOD is not enabled for this course",
		})
		return
	}

	// Step 5: Verify user is admin of the course
	if !user.IsAdminOfCourse(course) {
		logger.Warn("User not authorized for course", "userID", user.ID, "courseID", courseID)
		_ = c.Error(tools.RequestError{
			Status:        http.StatusForbidden,
			CustomMessage: "user is not authorized to upload to this course",
		})
		return
	}

	// Step 6: Parse request parameters (from query string)
	var req artemisUploadRequest
	
	// Check if uploading to existing stream
	streamIDStr := c.Query("streamID")
	if streamIDStr != "" {
		streamID, err := strconv.ParseUint(streamIDStr, 10, 32)
		if err == nil {
			req.StreamID = uint(streamID)
		}
	}

	// Get title from query parameters (required only for new streams)
	req.Title = c.Query("title")
	if req.Title == "" && req.StreamID == 0 {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusBadRequest,
			CustomMessage: "title is required when creating a new stream",
		})
		return
	}

	// Get optional parameters
	req.Description = c.Query("description")
	
	// Video type (default to COMB if not specified)
	videoTypeStr := c.Query("videoType")
	if videoTypeStr == "" {
		req.VideoType = model.VideoTypeCombined
	} else {
		req.VideoType = model.VideoType(videoTypeStr)
		if !req.VideoType.Valid() {
			_ = c.Error(tools.RequestError{
				Status:        http.StatusBadRequest,
				CustomMessage: "invalid video type (must be COMB, PRES, or CAM)",
			})
			return
		}
	}

	// Stream start time (default to now if not specified)
	streamStartStr := c.Query("streamStart")
	if streamStartStr == "" {
		req.StreamStart = time.Now()
	} else {
		parsedTime, err := time.Parse(time.RFC3339, streamStartStr)
		if err != nil {
			_ = c.Error(tools.RequestError{
				Status:        http.StatusBadRequest,
				CustomMessage: "invalid streamStart format (must be ISO 8601/RFC3339)",
				Err:           err,
			})
			return
		}
		req.StreamStart = parsedTime
	}

	// Step 7: Get or create stream
	var stream model.Stream
	
	if req.StreamID != 0 {
		// Upload to existing stream
		stream, err = r.StreamsDao.GetStreamByID(c, fmt.Sprintf("%d", req.StreamID))
		if err != nil {
			logger.Warn("Stream not found", "streamID", req.StreamID, "err", err)
			_ = c.Error(tools.RequestError{
				Status:        http.StatusNotFound,
				CustomMessage: "stream not found",
				Err:           err,
			})
			return
		}
		
		// Verify stream belongs to the course
		if stream.CourseID != course.ID {
			logger.Warn("Stream does not belong to course", "streamID", req.StreamID, "streamCourseID", stream.CourseID, "requestedCourseID", course.ID)
			_ = c.Error(tools.RequestError{
				Status:        http.StatusForbidden,
				CustomMessage: "stream does not belong to this course",
			})
			return
		}
		
		logger.Info("Uploading to existing stream", "streamID", stream.ID, "videoType", req.VideoType)
	} else {
		// Create a new stream/lecture for this upload
		stream = model.Stream{
			Name:        req.Title,
			Description: req.Description,
			CourseID:    course.ID,
			Start:       req.StreamStart,
			End:         req.StreamStart.Add(2 * time.Hour), // Default 2 hour duration
			Recording:   true,                               // Mark as recording/VOD
			StreamKey:   uuid.NewV4().String(),
			Premiere:    false,
			ChatEnabled: false,
		}

		err = r.StreamsDao.CreateStream(&stream)
		if err != nil {
			logger.Error("Failed to create stream", "err", err)
			_ = c.Error(tools.RequestError{
				Status:        http.StatusInternalServerError,
				CustomMessage: "failed to create stream",
				Err:           err,
			})
			return
		}

		logger.Info("Stream created", "streamID", stream.ID, "courseID", courseID, "userID", user.ID)
	}

	// Step 8: Create upload key for the worker
	uploadKey := uuid.NewV4().String()
	err = r.UploadKeyDao.CreateUploadKey(uploadKey, stream.ID, req.VideoType)
	if err != nil {
		logger.Error("Failed to create upload key", "err", err)
		_ = c.Error(tools.RequestError{
			Status:        http.StatusInternalServerError,
			CustomMessage: "failed to create upload key",
			Err:           err,
		})
		return
	}

	// Step 9: Get an available worker
	workers := r.WorkerDao.GetAliveWorkers()
	if len(workers) == 0 {
		logger.Error("No workers available")
		_ = c.Error(tools.RequestError{
			Status:        http.StatusServiceUnavailable,
			CustomMessage: "no workers available to process the video",
		})
		return
	}

	// Select worker with least workload
	worker := workers[getWorkerWithLeastWorkload(workers)]

	// Step 10: Build proxy URL to worker
	workerURL, err := url.Parse(fmt.Sprintf("http://%s:%s/upload", worker.Host, WorkerHTTPPort))
	if err != nil {
		logger.Error("Failed to parse worker URL", "err", err)
		_ = c.Error(tools.RequestError{
			Status:        http.StatusInternalServerError,
			CustomMessage: "failed to parse worker URL",
			Err:           err,
		})
		return
	}

	// Add query parameters for the worker
	query := workerURL.Query()
	query.Set("streamID", fmt.Sprintf("%d", stream.ID))
	query.Set("videoType", string(req.VideoType))
	query.Set("key", uploadKey)
	workerURL.RawQuery = query.Encode()

	logger.Info("Proxying upload to worker", "workerHost", worker.Host, "streamID", stream.ID)

	// Step 11: Proxy the upload request to the worker
	proxy := httputil.NewSingleHostReverseProxy(workerURL)
	proxy.Director = func(req *http.Request) {
		req.URL.Scheme = workerURL.Scheme
		req.URL.Host = workerURL.Host
		req.Host = workerURL.Host
		req.URL.Path = workerURL.Path
		req.URL.RawQuery = workerURL.RawQuery
	}

	// Build video URLs for Artemis to use
	baseURL := tools.Cfg.CanonicalURL
	if baseURL == "" {
		// Fallback to request host
		scheme := "https"
		if c.Request.TLS == nil {
			scheme = "http"
		}
		baseURL = scheme + "://" + c.Request.Host
	}

	// Video watch URL
	videoURL := fmt.Sprintf("%s/w/%s/%d", baseURL, course.Slug, stream.ID)
	
	// Embeddable URL (with video_only parameter for iframe)
	embedURL := fmt.Sprintf("%s/w/%s/%d?video_only=1", baseURL, course.Slug, stream.ID)

	// Add custom response headers for Artemis
	c.Header("X-Stream-ID", fmt.Sprintf("%d", stream.ID))
	c.Header("X-Course-Slug", course.Slug)
	c.Header("X-Video-URL", videoURL)
	c.Header("X-Embed-URL", embedURL)

	// Proxy the request
	proxy.ServeHTTP(c.Writer, c.Request)

	logger.Info("Upload successfully proxied", "streamID", stream.ID, "courseSlug", course.Slug, "videoURL", videoURL)
}

// authenticateByUserToken validates the User API token and returns the associated user
func (r artemisRoutes) authenticateByUserToken(c *gin.Context) (*model.User, error) {
	// Try to get token from Authorization header first (Bearer token)
	tokenString := ""
	authHeader := c.GetHeader("Authorization")
	if authHeader != "" {
		// Format: "Bearer <token>"
		const bearerPrefix = "Bearer "
		if len(authHeader) > len(bearerPrefix) && authHeader[:len(bearerPrefix)] == bearerPrefix {
			tokenString = authHeader[len(bearerPrefix):]
		}
	}

	// Fall back to query parameter if not in header
	if tokenString == "" {
		tokenString = c.Query("token")
	}

	if tokenString == "" {
		return nil, fmt.Errorf("missing authentication token")
	}

	// Get token from database (this automatically checks expiration)
	token, err := r.TokenDao.GetToken(tokenString)
	if err != nil {
		return nil, fmt.Errorf("invalid or expired token: %w", err)
	}

	// Verify token has appropriate scope (lecturer or admin)
	if token.Scope != model.TokenScopeLecturer && token.Scope != model.TokenScopeAdmin {
		return nil, fmt.Errorf("token does not have required scope")
	}

	// Get the user associated with this token
	user, err := r.UsersDao.GetUserByID(context.Background(), token.UserID)
	if err != nil {
		return nil, fmt.Errorf("user not found for token: %w", err)
	}

	// Mark token as used (for audit purposes)
	err = r.TokenDao.TokenUsed(token)
	if err != nil {
		logger.Warn("Failed to mark token as used", "err", err)
		// Don't fail the request if we can't update last_use
	}

	logger.Info("Token authenticated", "userID", user.ID, "userName", user.Name, "tokenID", token.ID)

	return &user, nil
}



