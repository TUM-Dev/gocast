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

// GenericInternalErrorMessage is what a client is told when a handler pushes a plain
// error instead of a RequestError. Raw Go error strings routinely carry SQL fragments,
// file paths and internal hostnames, so the real cause is logged rather than returned.
const GenericInternalErrorMessage = "internal server error"

func ErrorHandler(c *gin.Context) {
	c.Next()
	if len(c.Errors) == 0 {
		return
	}

	err := c.Errors[0]
	c.Errors = []*gin.Error{} // clear errors so they don't get logged twice

	switch tErr := err.Err.(type) {
	case RequestError:
		// Deliberately constructed by a handler, so its message and cause are meant
		// for the client.
		c.JSON(tErr.Status, tErr.ToResponse())
	default:
		// Unexpected error: keep it server-side and hand the client the same shape
		// with a generic message.
		logger.Error("unhandled error while serving request",
			"err", err.Err,
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
		)
		c.JSON(http.StatusInternalServerError, RequestError{
			Status:        http.StatusInternalServerError,
			CustomMessage: GenericInternalErrorMessage,
		}.ToResponse())
	}
	c.Abort()
}
