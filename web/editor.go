package web

import (
	"github.com/gin-gonic/gin"
	"github.com/joschahenningsen/TUM-Live/model"
	"github.com/joschahenningsen/TUM-Live/tools"
)

func (r mainRoutes) editorPage(c *gin.Context) {
	tlctx := c.MustGet("TUMLiveContext").(tools.TUMLiveContext)
	id := NewIndexData()
	id.IsUser = true
	id.IsAdmin = true
	id.UserName = tlctx.User.Name
	id.TUMLiveContext = tlctx
	err := templateExecutor.ExecuteTemplate(c.Writer, "editor.gohtml", editorPageData{
		IndexData: id,
		Course:    *tlctx.Course,
		Stream:    *tlctx.Stream,
	})
	if err != nil {
		c.Writer.Write([]byte(err.Error()))
	}
}

type editorPageData struct {
	IndexData IndexData
	Course    model.Course
	Stream    model.Stream
}
