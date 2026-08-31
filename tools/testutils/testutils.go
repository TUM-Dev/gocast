package testutils

import (
	"encoding/json"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"github.com/TUM-Dev/gocast/tools"
)

func Equal(t *testing.T, a, b interface{}) {
	av, okA := a.([]byte)
	bv, okB := b.([]byte)

	if okA && okB {
		if json.Valid(av) && json.Valid(bv) {
			assert.JSONEq(t, string(av), string(bv))
			return
		}
	}
}

func GetMiddlewares(mw ...func(ctx *gin.Context)) []func(c *gin.Context) {
	return mw
}

func TUMLiveContext(ctx tools.TUMLiveContext) func(c *gin.Context) {
	return func(c *gin.Context) {
		c.Set("TUMLiveContext", ctx)
	}
}
