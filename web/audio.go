package web

import (
	"github.com/gin-gonic/gin"
	"log"
)

func (r mainRoutes) AudioPage(c *gin.Context) {
	data := AudioPageData{NewIndexData()}
	err := templateExecutor.ExecuteTemplate(c.Writer, "audio.gohtml", data)
	if err != nil {
		log.Printf("couldn't render template: %v\n", err)
	}
}

type AudioPageData struct {
	IndexData
}
