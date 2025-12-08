package api

import (
	"database/sql"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	uuid "github.com/satori/go.uuid"

	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/model"
	"github.com/TUM-Dev/gocast/tools"
	"github.com/TUM-Dev/gocast/tools/tum"
)

func configTokenRouter(r *gin.Engine, daoWrapper dao.DaoWrapper) {
	routes := tokenRoutes{daoWrapper}
	g := r.Group("/api/token")
	g.POST("/proxy/:token", routes.fetchStreamKey)
	g.Use(tools.AtLeastLecturer)
	g.POST("/create", routes.createToken)
	g.DELETE("/:id", routes.deleteToken)
	g.GET("/artemis", routes.getArtemisToken)
	g.GET("/artemis/courses", routes.getArtemisCourses)
}

type tokenRoutes struct {
	dao.DaoWrapper
}

func (r tokenRoutes) deleteToken(c *gin.Context) {
	id := c.Param("id")

	foundContext, exists := c.Get("TUMLiveContext")
	if !exists {
		return
	}
	tumLiveContext := foundContext.(tools.TUMLiveContext)

	token, err := r.TokenDao.GetTokenByID(id)
	if err != nil {
		logger.Error("can not get token", "err", err)
		_ = c.Error(tools.RequestError{
			Status:        http.StatusInternalServerError,
			CustomMessage: "can not get token",
			Err:           err,
		})
		return
	}

	// only the user who created the token or an admin can delete it
	if token.UserID != tumLiveContext.User.ID && tumLiveContext.User.Role != model.AdminType {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusForbidden,
			CustomMessage: "not allowed to delete token",
		})
		return
	}

	err = r.TokenDao.DeleteToken(id)
	if err != nil {
		logger.Error("can not delete token", "err", err)
		_ = c.Error(tools.RequestError{
			Status:        http.StatusInternalServerError,
			CustomMessage: "can not delete token",
			Err:           err,
		})
		return
	}
}

func (r tokenRoutes) createToken(c *gin.Context) {
	foundContext, exists := c.Get("TUMLiveContext")
	if !exists {
		return
	}
	tumLiveContext := foundContext.(tools.TUMLiveContext)

	var req struct {
		Expires *time.Time `json:"expires"`
		Scope   string     `json:"scope"`
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
	if req.Scope == model.TokenScopeAdmin && tumLiveContext.User.Role != model.AdminType {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusBadRequest,
			CustomMessage: "not an admin",
		})
		return
	}

	if req.Scope != model.TokenScopeAdmin && req.Scope != model.TokenScopeLecturer {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusBadRequest,
			CustomMessage: "invalid scope",
		})
		return
	}

	tokenStr := uuid.NewV4().String()
	expires := sql.NullTime{Valid: req.Expires != nil}
	if req.Expires != nil {
		expires.Time = *req.Expires
	}
	token := model.Token{
		UserID:  tumLiveContext.User.ID,
		Token:   tokenStr,
		Expires: expires,
		Scope:   req.Scope,
	}
	err = r.TokenDao.AddToken(token)
	if err != nil {
		logger.Error("can not create token", "err", err)
		_ = c.Error(tools.RequestError{
			Status:        http.StatusInternalServerError,
			CustomMessage: "can not create token",
			Err:           err,
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"token": tokenStr,
	})
}

// This is used by the proxy to get the stream key of the next stream of the lecturer given a lecturer token
//
//	Proxy receives: rtmp://proxy.example.com/<lecturer-token>
//				or: rtmp://proxy.example.com/<lecturer-token>?slug=ABC-123 <-- optional slug parameter in case the lecturer is streaming multiple courses simultaneously
//
//	Proxy returns:  rtmp://ingest.example.com/ABC-123?secret=610f609e4a2c43ac8a6d648177472b17
func (r *tokenRoutes) fetchStreamKey(c *gin.Context) {
	// Optional slug parameter to get the stream key of a specific course (in case the lecturer is streaming multiple courses simultaneously)
	slug := c.Query("slug")
	t := c.Param("token")

	// Get user from token
	token, err := r.TokenDao.GetToken(t)
	if err != nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusBadRequest,
			CustomMessage: "invalid token",
		})
		return
	}

	// Only tokens of type lecturer are allowed to start streaming
	if token.Scope != model.TokenScopeLecturer {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusUnauthorized,
			CustomMessage: "invalid scope",
		})
		return
	}

	// Get user and check if he has the right to start a stream
	user, err := r.UsersDao.GetUserByID(c, token.UserID)
	if err != nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusInternalServerError,
			CustomMessage: "could not get user",
			Err:           err,
		})
		return

	}
	if user.Role != model.LecturerType && user.Role != model.AdminType {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusUnauthorized,
			CustomMessage: "user is not a lecturer or admin",
		})
		return
	}

	// Find current/next stream and course of which the user is a lecturer
	year, term := tum.GetCurrentSemester()
	streamKey, courseSlug, err := r.StreamsDao.GetSoonStartingStreamInfo(&user, slug, year, term)
	if err != nil || streamKey == "" || courseSlug == "" {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusNotFound,
			CustomMessage: "no stream found",
			Err:           err,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{"url": "" + tools.Cfg.IngestBase + courseSlug + "?secret=" + streamKey + "/" + courseSlug})
}

// getArtemisToken returns or creates an upload token for the logged-in user for Artemis integration
func (r tokenRoutes) getArtemisToken(c *gin.Context) {
	foundContext, exists := c.Get("TUMLiveContext")
	if !exists {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusUnauthorized,
			CustomMessage: "not logged in",
		})
		return
	}
	tumLiveContext := foundContext.(tools.TUMLiveContext)

	if tumLiveContext.User == nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusUnauthorized,
			CustomMessage: "not logged in",
		})
		return
	}

	// Check if user already has an active Artemis upload token
	allTokens, err := r.TokenDao.GetAllTokens(tumLiveContext.User)
	if err != nil {
		logger.Error("can not get tokens", "err", err)
		_ = c.Error(tools.RequestError{
			Status:        http.StatusInternalServerError,
			CustomMessage: "can not get tokens",
			Err:           err,
		})
		return
	}

	// Look for an active lecturer token
	var existingToken *model.Token
	for _, tokenDto := range allTokens {
		// Check if it's a lecturer token and not expired
		if tokenDto.Scope == model.TokenScopeLecturer {
			if tokenDto.Expires.Valid && tokenDto.Expires.Time.Before(time.Now()) {
				// Token is expired, skip it
				continue
			}
			// Found a valid token
			token := tokenDto.Token  // Extract the embedded model.Token
			existingToken = &token
			break
		}
	}

	// If no valid token exists, create one (valid for 1 year)
	if existingToken == nil {
		tokenStr := uuid.NewV4().String()
		expires := sql.NullTime{
			Valid: true,
			Time:  time.Now().AddDate(1, 0, 0), // 1 year from now
		}
		newToken := model.Token{
			UserID:  tumLiveContext.User.ID,
			Token:   tokenStr,
			Expires: expires,
			Scope:   model.TokenScopeLecturer,
		}
		err = r.TokenDao.AddToken(newToken)
		if err != nil {
			logger.Error("can not create token", "err", err)
			_ = c.Error(tools.RequestError{
				Status:        http.StatusInternalServerError,
				CustomMessage: "can not create token",
				Err:           err,
			})
			return
		}
		existingToken = &newToken
	}

	// Get courses where user is admin
	courses, err := r.CoursesDao.GetAdministeredCoursesByUserId(c, tumLiveContext.User.ID, "", 0)
	if err != nil {
		logger.Error("can not get courses", "err", err)
		_ = c.Error(tools.RequestError{
			Status:        http.StatusInternalServerError,
			CustomMessage: "can not get courses",
			Err:           err,
		})
		return
	}

	// Filter only courses with VOD enabled
	vodEnabledCourses := []gin.H{}
	for _, course := range courses {
		if course.VODEnabled {
			vodEnabledCourses = append(vodEnabledCourses, gin.H{
				"id":   course.ID,
				"name": course.Name,
				"slug": course.Slug,
				"year": course.Year,
				"term": course.TeachingTerm,
			})
		}
	}

	// Return token and course list
	c.JSON(http.StatusOK, gin.H{
		"token":   existingToken.Token,
		"expires": existingToken.Expires,
		"courses": vodEnabledCourses,
		"baseUrl": "https://" + c.Request.Host,
		"usage": gin.H{
			"endpoint": "/api/course/artemis/{courseID}/upload",
			"example":  "https://" + c.Request.Host + "/api/course/artemis/{courseID}/upload?title=My+Video",
		},
	})
}

// getArtemisCourses returns the list of courses where the user can upload videos
func (r tokenRoutes) getArtemisCourses(c *gin.Context) {
	foundContext, exists := c.Get("TUMLiveContext")
	if !exists {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusUnauthorized,
			CustomMessage: "not logged in",
		})
		return
	}
	tumLiveContext := foundContext.(tools.TUMLiveContext)

	if tumLiveContext.User == nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusUnauthorized,
			CustomMessage: "not logged in",
		})
		return
	}

	// Get courses where user is admin
	courses, err := r.CoursesDao.GetAdministeredCoursesByUserId(c, tumLiveContext.User.ID, "", 0)
	if err != nil {
		logger.Error("can not get courses", "err", err)
		_ = c.Error(tools.RequestError{
			Status:        http.StatusInternalServerError,
			CustomMessage: "can not get courses",
			Err:           err,
		})
		return
	}

	// Filter only courses with VOD enabled
	vodEnabledCourses := []gin.H{}
	for _, course := range courses {
		if course.VODEnabled {
			vodEnabledCourses = append(vodEnabledCourses, gin.H{
				"id":          course.ID,
				"name":        course.Name,
				"slug":        course.Slug,
				"year":        course.Year,
				"term":        course.TeachingTerm,
				"uploadUrl":   "https://" + c.Request.Host + "/api/course/artemis/" + fmt.Sprintf("%d", course.ID) + "/upload",
				"description": fmt.Sprintf("%s (%s %d)", course.Name, course.TeachingTerm, course.Year),
			})
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"courses": vodEnabledCourses,
		"total":   len(vodEnabledCourses),
	})
}
