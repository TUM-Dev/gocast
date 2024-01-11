package testutils

import (
	"github.com/TUM-Dev/gocast/tools"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"testing"
)

func Equal(t *testing.T, a, b interface{}) {
	assert.Equal(t, a, b)
}

func GetMiddlewares(mw ...func(ctx *gin.Context)) []func(c *gin.Context) {
	return mw
}

func TUMLiveContext(ctx tools.TUMLiveContext) func(c *gin.Context) {
	return func(c *gin.Context) {
		c.Set("TUMLiveContext", ctx)
	}
}
