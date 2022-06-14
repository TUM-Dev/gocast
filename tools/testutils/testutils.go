package testutils

import (
	"github.com/gin-gonic/gin"
	"github.com/joschahenningsen/TUM-Live/dao"
	"github.com/joschahenningsen/TUM-Live/tools"
	"github.com/stretchr/testify/assert"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

type TestCases map[string]TestCase

type TestCase struct {
	Method           string
	Url              string
	DaoWrapper       dao.DaoWrapper
	TumLiveContext   *tools.TUMLiveContext
	Body             io.Reader
	ExpectedCode     int
	ExpectedResponse []byte
}

func (tc TestCases) Run(t *testing.T, configRouterFunc func(*gin.Engine, dao.DaoWrapper)) {
	for name, testCase := range tc {
		t.Run(name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, r := gin.CreateTestContext(w)

			if testCase.TumLiveContext != nil {
				r.Use(func(c *gin.Context) {
					c.Set("TUMLiveContext", *testCase.TumLiveContext)
				})
			}

			configRouterFunc(r, testCase.DaoWrapper)

			c.Request, _ = http.NewRequest(testCase.Method, testCase.Url, testCase.Body)
			r.ServeHTTP(w, c.Request)

			assert.Equal(t, testCase.ExpectedCode, w.Code)

			if len(testCase.ExpectedResponse) > 0 {
				assert.Equal(t, string(testCase.ExpectedResponse), w.Body.String())
			}
		})
	}
}

func First(a interface{}, b interface{}) interface{} {
	return a
}

func Second(a interface{}, b interface{}) interface{} {
	return b
}
