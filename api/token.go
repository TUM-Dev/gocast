package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/model"
	"github.com/TUM-Dev/gocast/tools"
	"github.com/TUM-Dev/gocast/tools/tum"
)

// Issuing and revoking tokens moved to apiv2/server/token_admin.go with the
// administration page they belonged to; this route is what is left, and it serves
// the RTMP proxy rather than a browser, so it stays in v1.
func configTokenRouter(r *gin.Engine, daoWrapper dao.DaoWrapper) {
	routes := tokenRoutes{daoWrapper}
	g := r.Group("/api/token")
	g.POST("/proxy/:token", routes.fetchStreamKey)
}

type tokenRoutes struct {
	dao.DaoWrapper
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
	if !user.Can(model.PermLecture) {
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
