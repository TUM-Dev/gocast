package tools

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type RequestError struct {
	Status        int
	CustomMessage string
	Err           error
}

func (r RequestError) Error() string {
	if r.Err != nil {
		return r.Err.Error()
	}
	return ""
}

func (r RequestError) ToResponse() gin.H {
	res := gin.H{"status": r.Status, "message": r.CustomMessage}

	if r.Err != nil {
		res["error"] = r.Error()
	}

	return res
}

func ErrorHandler(c *gin.Context) {
	c.Next()
	if len(c.Errors) > 0 {
		err := c.Errors[0]
		switch tErr := err.Err.(type) {
		case RequestError:
			c.Errors = []*gin.Error{} // clear errors so they don't get logged
			c.JSON(tErr.Status, tErr.ToResponse())
		default:
			c.Errors = []*gin.Error{} // clear errors so they don't get logged
			c.JSON(http.StatusInternalServerError, err.Err.Error())
		}
		c.Abort()
		return
	}
}
