package api

import (
	"github.com/gin-gonic/gin"
	"github.com/joschahenningsen/TUM-Live/dao"
	"github.com/joschahenningsen/TUM-Live/model"
	"github.com/joschahenningsen/TUM-Live/tools"
	"net/http"
)

type auditRoutes struct {
	dao.DaoWrapper
}

func configAuditRouter(r *gin.Engine, d dao.DaoWrapper) {
	auditRouter := auditRoutes{d}
	g := r.Group("/api/audits")
	g.Use(tools.Admin)
	{
		g.GET("/", auditRouter.getAudits)
	}
}

func (r auditRoutes) getAudits(c *gin.Context) {
	type req struct {
		Limit  int               `form:"limit"`
		Offset int               `form:"offset"`
		Types  []model.AuditType `form:"types[]"`
	}
	var reqData req
	err := c.BindQuery(&reqData)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, err.Error())
		return
	}
	found, err := r.AuditDao.Find(reqData.Limit, reqData.Offset, reqData.Types...)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, found)
}
