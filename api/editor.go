package api

import (
	"TUM-Live/dao"
	"TUM-Live/tools"
	"fmt"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"net/http"
	"os"
	"time"
)

func configOpencastReceiverRoute(router *gin.Engine) {
	// opencast needs cors capabilities
	g := router.Group("/api")
	g.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE, UPDATE")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")
	})
	g.OPTIONS("/editor/edit.json", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	g.GET("/editor/edit.json", getEditJson)
	g.OPTIONS("/editor/metadata.json", func(c *gin.Context) {
		log.Info(c.Request.URL.Path)
		c.Status(http.StatusOK)
	})
	g.GET("/editor/metadata.json", func(c *gin.Context) {
		c.Header("Content-Type", "application/json")
		c.Writer.Write([]byte(`[{"flavor":"dublincore/episode","title":"EVENTS.EVENTS.DETAILS.CATALOG.EPISODE","fields":[{"readOnly":false,"id":"duration","label":"EVENTS.EVENTS.DETAILS.METADATA.DURATION","type":"text","value":"09:09:57","required":false},{"readOnly":true,"id":"identifier","label":"EVENTS.EVENTS.DETAILS.METADATA.ID","type":"text","value":"ID-dual-stream-demo","required":false}]}]`))
	})
	g.POST("/editor/edit.json", func(context *gin.Context) {
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

func getEditJson(c *gin.Context) {
	sid, ok := c.GetQuery("stream")
	if !ok {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	stream, err := dao.GetStreamByID(c, sid)
	if err != nil {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}

	c.JSON(http.StatusOK, editData{
		Segments: []interface{}{},
		Workflows: []workflow{{
			ID:           "publish",
			Name:         "Publish",
			DisplayOrder: 1000,
		}},
		Tracks: []track{
			{
				AudioStream: audioStream{
					Available:    true,
					ThumbnailURI: nil,
					Enabled:      true,
				},
				VideoStream: videoStream{
					Available:    true,
					ThumbnailURI: nil,
					Enabled:      true,
				},
				Flavor: flavor{
					Type:    "presenter",
					Subtype: "preview",
				},
				URI: fmt.Sprintf("http://localhost:8081/content/%d/%d/file.mp4", stream.CourseID, stream.ID),
				ID:  fmt.Sprintf("%d", stream.ID),
			},
		},

		Title:          "Edit Video",
		Date:           stream.Start,
		Duration:       stream.Duration * 1000, // seconds to milliseconds
		Series:         Series{},
		WorkflowActive: false,
	})
}

type editData struct {
	Segments       []interface{} `json:"segments"`
	Workflows      []workflow    `json:"workflows"`
	Tracks         []track       `json:"tracks"`
	Title          string        `json:"title"`
	Date           time.Time     `json:"date"`
	Duration       uint32        `json:"duration"`
	Series         Series        `json:"series"`
	WorkflowActive bool          `json:"workflow_active"`
}
type workflow struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	DisplayOrder int    `json:"displayOrder"`
	Description  string `json:"description"`
}
type audioStream struct {
	Available    bool        `json:"available"`
	ThumbnailURI interface{} `json:"thumbnail_uri"`
	Enabled      bool        `json:"enabled"`
}
type videoStream struct {
	Available    bool        `json:"available"`
	ThumbnailURI interface{} `json:"thumbnail_uri"`
	Enabled      bool        `json:"enabled"`
}
type flavor struct {
	Type    string `json:"type"`
	Subtype string `json:"subtype"`
}
type track struct {
	AudioStream audioStream `json:"audio_stream"`
	VideoStream videoStream `json:"video_stream"`
	Flavor      flavor      `json:"flavor"`
	URI         string      `json:"uri"`
	ID          string      `json:"id"`
}
type Series struct {
	ID    interface{} `json:"id"`
	Title interface{} `json:"title"`
}
