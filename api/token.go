package api

import (
	"TUM-Live/dao"
	"TUM-Live/model"
	"TUM-Live/tools"
	"database/sql"
	"github.com/gin-gonic/gin"
	uuid "github.com/satori/go.uuid"
	log "github.com/sirupsen/logrus"
	"net/http"
	"time"
)

func configTokenRouter(r *gin.Engine) {
	g := r.Group("/api/token")
	g.Use(tools.Admin)
	g.POST("/create", createToken)
	g.DELETE("/:id", deleteToken)
}

func deleteToken(c *gin.Context) {
	id := c.Param("id")
	err := dao.DeleteToken(id)
	if err != nil {
		log.WithError(err).Error("delete token failed")
		c.AbortWithStatus(http.StatusInternalServerError)
	}
}

func createToken(c *gin.Context) {
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
	log.Println(req)
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
	err = dao.AddToken(token)
	if err != nil {
		log.WithError(err).Error("Failed to create token")
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"token": tokenStr,
	})
}
