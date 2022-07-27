package tools

import (
	"github.com/gin-gonic/gin"
	"net/http"
)

type RequestError struct {
	Status        int
	CustomMessage string
	Err           error
}

func (r RequestError) Error() string {
	if r.Err != nil {
		return r.Err.Error()
	} else {
		return ""
	}
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
		switch err.Err.(type) {
		case RequestError:
			e := err.Err.(RequestError)
			c.Errors = []*gin.Error{} // clear errors so they don't get logged
			c.JSON(e.Status, e.ToResponse())
		default:
			c.Errors = []*gin.Error{} // clear errors so they don't get logged
			c.JSON(http.StatusInternalServerError, err.Err.Error())
		}
		c.Abort()
		return
	}
}
