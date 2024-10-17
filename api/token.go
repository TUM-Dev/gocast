package api

import (
	"database/sql"
	"net/http"
	"time"

	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/model"
	"github.com/TUM-Dev/gocast/tools"
	"github.com/TUM-Dev/gocast/tools/tum"
	"github.com/gin-gonic/gin"
	uuid "github.com/satori/go.uuid"
)

func configTokenRouter(r *gin.Engine, daoWrapper dao.DaoWrapper) {
	routes := tokenRoutes{daoWrapper}
	g := r.Group("/api/token")
	g.Use(tools.AtLeastLecturer)
	g.POST("/create", routes.createToken)
	g.DELETE("/:id", routes.deleteToken)
	g.POST("/:token", routes.fetchStreamKey)
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
func (s *tokenRoutes) fetchStreamKey(c *gin.Context) {
	// Optional slug parameter to get the stream key of a specific course (in case the lecturer is streaming multiple courses simultaneously)
	slug := c.Query("slug")
	t := c.Param("token")

	// Get user from token
	token, err := s.TokenDao.GetToken(t)
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
	user, err := s.UsersDao.GetUserByID(c, token.UserID)
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
	streamKey, courseSlug, err := s.StreamsDao.GetSoonStartingStreamInfo(&user, slug, year, term)
	if err != nil || streamKey == "" || courseSlug == "" {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusNotFound,
			CustomMessage: "no stream found",
			Err:           err,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{"url": "" + tools.Cfg.IngestBase + "/" + courseSlug + "?secret=" + streamKey + "/" + courseSlug})
}
