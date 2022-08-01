package api

import (
	"database/sql"
	"github.com/gin-gonic/gin"
	"github.com/joschahenningsen/TUM-Live/dao"
	"github.com/joschahenningsen/TUM-Live/model"
	"github.com/joschahenningsen/TUM-Live/tools"
	uuid "github.com/satori/go.uuid"
	log "github.com/sirupsen/logrus"
	"net/http"
	"time"
)

func configTokenRouter(r *gin.Engine, daoWrapper dao.DaoWrapper) {
	routes := tokenRoutes{daoWrapper}
	g := r.Group("/api/token")
	g.Use(tools.Admin)
	g.POST("/create", routes.createToken)
	g.DELETE("/:id", routes.deleteToken)
}

type tokenRoutes struct {
	dao.DaoWrapper
}

func (r tokenRoutes) deleteToken(c *gin.Context) {
	id := c.Param("id")
	err := r.TokenDao.DeleteToken(c, id)
	if err != nil {
		log.WithError(err).Error("can not delete token")
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
	if req.Scope != model.TokenScopeAdmin {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusBadRequest,
			CustomMessage: "not an admin",
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
	err = r.TokenDao.AddToken(c, token)
	if err != nil {
		log.WithError(err).Error("can not create token")
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
