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
	err := r.TokenDao.DeleteToken(id)
	if err != nil {
		log.WithError(err).Error("delete token failed")
		c.AbortWithStatus(http.StatusInternalServerError)
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
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	if req.Scope != model.TokenScopeAdmin {
		c.AbortWithStatus(http.StatusBadRequest)
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
		log.WithError(err).Error("Failed to create token")
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"token": tokenStr,
	})
}
