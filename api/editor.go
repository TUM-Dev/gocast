package api

import (
	"github.com/gin-gonic/gin"
	"github.com/joschahenningsen/TUM-Live/dao"
	"github.com/joschahenningsen/TUM-Live/tools"
	log "github.com/sirupsen/logrus"
	"net/http"
	"time"
)

type editorRoutes struct {
	dao.DaoWrapper
}

func configEditorRouter(r *gin.Engine, wrapper dao.DaoWrapper) {
	routes := editorRoutes{wrapper}
	g := r.Group("/api/editor")
	g.Use(cors())
	{
		g.OPTIONS("/metadata.json", routes.getMetadata)
		g.GET("/metadata.json", routes.getMetadata)
		g.OPTIONS("/edit.json", routes.getEdit)
		g.GET("/edit.json", routes.getEdit)
	}
}

func (r editorRoutes) getEdit(c *gin.Context) {
	log.Info(c.MustGet("TUMLiveContext").(tools.TUMLiveContext).User)
	c.JSON(http.StatusOK, edit{})
}
func (r editorRoutes) getMetadata(c *gin.Context) {
	c.JSON(http.StatusOK, metadata{})
}

func cors() gin.HandlerFunc {
	return func(c *gin.Context) {

		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Credentials", "true")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Header("Access-Control-Allow-Methods", "POST,HEAD,PATCH, OPTIONS, GET, PUT")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}

/**
* types for edit endpoint:
 */

type edit struct {
	Segments       []Segments  `json:"segments"`
	Workflows      []Workflows `json:"workflows"`
	Tracks         []Tracks    `json:"tracks"`
	Title          string      `json:"title"`
	Date           time.Time   `json:"date"`
	Duration       int         `json:"duration"`
	Series         Series      `json:"series"`
	WorkflowActive bool        `json:"workflow_active"`
}
type Segments struct {
	Start   int  `json:"start"`
	End     int  `json:"end"`
	Deleted bool `json:"deleted"`
}
type Workflows struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	DisplayOrder int    `json:"displayOrder"`
	Description  string `json:"description"`
}
type AudioStream struct {
	Available    bool        `json:"available"`
	ThumbnailURI interface{} `json:"thumbnail_uri"`
	Enabled      bool        `json:"enabled"`
}
type VideoStream struct {
	Available    bool        `json:"available"`
	ThumbnailURI interface{} `json:"thumbnail_uri"`
	Enabled      bool        `json:"enabled"`
}
type Flavor struct {
	Type    string `json:"type"`
	Subtype string `json:"subtype"`
}
type Tracks struct {
	AudioStream AudioStream `json:"audio_stream"`
	VideoStream VideoStream `json:"video_stream"`
	Flavor      Flavor      `json:"flavor"`
	URI         string      `json:"uri"`
	ID          string      `json:"id"`
}
type Series struct {
	ID    interface{} `json:"id"`
	Title interface{} `json:"title"`
}

/**
* types for metadata endpoint:
 */

type metadata []struct {
	Flavor string   `json:"flavor"`
	Title  string   `json:"title"`
	Fields []Fields `json:"fields"`
}
type Collection struct {
	LANGUAGESENGLISH string `json:"LANGUAGES.ENGLISH"`
	LANGUAGESGERMAN  string `json:"LANGUAGES.GERMAN"`
}

type Fields struct {
	ReadOnly     bool       `json:"readOnly"`
	ID           string     `json:"id"`
	Label        string     `json:"label"`
	Type         string     `json:"type"`
	Value        string     `json:"value"`
	Required     bool       `json:"required"`
	Translatable bool       `json:"translatable,omitempty"`
	Collection   Collection `json:"collection,omitempty"`
}
