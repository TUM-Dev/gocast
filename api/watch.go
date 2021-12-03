package api

import (
	"TUM-Live/tools"
	"github.com/gin-gonic/gin"
	"net/http"
	"time"
)

func configWatchRouter(e *gin.Engine) {
	g := e.Group("/api/watch")
	g.Use(tools.InitStream)
	g.GET("/:streamID", watch)
}

func watch(ctx *gin.Context) {
	context, found := ctx.Get("TUMLiveContext")
	if !found {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Context not found"})
		return
	}
	tumLiveContext := context.(tools.TUMLiveContext)
	s := tumLiveContext.Stream
	c := tumLiveContext.Course
	if s == nil {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "Stream not found"})
		return
	}
	if c == nil {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "Course not found"})
		return
	}
	var playlists []PlaylistDto
	if s.PlaylistUrl != "" {
		playlists = append(playlists, PlaylistDto{Type: "COMB", URL: s.PlaylistUrl})
	}
	if s.PlaylistUrlPRES != "" {
		playlists = append(playlists, PlaylistDto{Type: "PRES", URL: s.PlaylistUrlPRES})
	}
	if s.PlaylistUrlCAM != "" {
		playlists = append(playlists, PlaylistDto{Type: "CAM", URL: s.PlaylistUrlCAM})
	}
	ctx.JSON(http.StatusOK, WatchDto{
		Stream: StreamDto{
			Id:          s.ID,
			Name:        s.Name,
			Start:       s.Start,
			End:         s.End,
			Description: s.GetDescriptionHTML(),
			Playlists:   playlists,
		},
		Course: CourseDto{
			Id:          c.ID,
			Slug:        c.Slug,
			ChatEnabled: c.ChatEnabled,
			Title:       c.Name,
			Streams:     nil,
		},
	})
}

type WatchDto struct {
	Stream StreamDto `json:"stream"`
	Course CourseDto `json:"course"`
}

type CourseDto struct {
	Id          uint        `json:"id"`
	Slug        string      `json:"slug"`
	ChatEnabled bool        `json:"chatEnabled"`
	Title       string      `json:"title"`
	Streams     []StreamDto `json:"streams"`
}

type StreamDto struct {
	Id          uint          `json:"id"`
	Name        string        `json:"name"`
	Start       time.Time     `json:"start"`
	End         time.Time     `json:"end"`
	Description string        `json:"description"`
	Playlists   []PlaylistDto `json:"playlists"`
}

type PlaylistDto struct {
	Type string `json:"type"`
	URL  string `json:"url"`
}
