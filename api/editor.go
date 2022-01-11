package api

import (
	"TUM-Live/dao"
	"TUM-Live/tools"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"net/http"
	"os"
	"time"
)

func configOpencastReceiverRoute(router *gin.Engine) {
	// opencast needs cors capabilities
	g := router.Group("/metadata")
	g.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE, UPDATE")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")
	})
	g.OPTIONS("/editor/:id/edit.json", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	g.POST("/editor/:id/edit.json", func(context *gin.Context) {
		req := struct {
			Segments []struct {
				Start    int  `json:"start"`
				End      int  `json:"end"`
				Deleted  bool `json:"deleted"`
				Selected bool `json:"selected"`
			} `json:"segments"`
		}{}
		err := context.BindJSON(&req)
		if err != nil {
			context.AbortWithStatus(http.StatusBadRequest)
			return
		}
		log.Println(req)
	})

	gdl := router.Group("/content/:courseID/:streamID")
	gdl.Use(tools.AdminOfCourse)
	gdl.GET("/file.mp4", func(context *gin.Context) {
		stream, err := dao.GetStreamByID(context, context.Param("streamID"))
		if err != nil {
			context.AbortWithStatus(http.StatusNotFound)
			return
		}
		file, err := os.OpenFile(stream.Files[0].Path, os.O_RDONLY, 0666)
		if err != nil {
			context.AbortWithStatus(http.StatusNotFound)
			return
		}
		defer func() {
			_ = file.Close()
		}()
		context.Header("Content-Type", "video/mp4")
		http.ServeContent(context.Writer, context.Request, "file.mp4", time.Now(), file)
	})
}
