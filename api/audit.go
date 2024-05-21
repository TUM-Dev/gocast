package api

import (
	"errors"
	"github.com/TUM-Dev/gocast/tools/oauth"
	"net/http"

	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/model"
	"github.com/TUM-Dev/gocast/tools"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type auditRoutes struct {
	dao.DaoWrapper
}

func configAuditRouter(r *gin.Engine, d dao.DaoWrapper) {
	auditRouter := auditRoutes{d}
	g := r.Group("/api")
	g.Use(oauth.Admin)
	{
		g.GET("/audits", auditRouter.getAudits)
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
		_ = c.Error(tools.RequestError{
			Status:        http.StatusBadRequest,
			CustomMessage: "can not bind query",
			Err:           err,
		})
		return
	}
	if len(reqData.Types) == 0 {
		reqData.Types = model.GetAllAuditTypes()
	}
	found, err := r.AuditDao.Find(reqData.Limit, reqData.Offset, reqData.Types...)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusInternalServerError,
			CustomMessage: "can not find audits",
			Err:           err,
		})
		return
	}
	res := make([]gin.H, len(found))
	for i := range res {
		res[i] = found[i].Json()
	}
	c.JSON(http.StatusOK, res)
}
