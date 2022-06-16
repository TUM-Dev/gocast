package testutils

import (
	"bytes"
	"github.com/gin-gonic/gin"
	"github.com/joschahenningsen/TUM-Live/dao"
	"github.com/joschahenningsen/TUM-Live/tools"
	"github.com/stretchr/testify/assert"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
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

func NewFormBody(values map[string]string) []byte {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	for k, v := range values {
		fw, _ := writer.CreateFormField(k)
		_, _ = io.Copy(fw, strings.NewReader(v))
	}
	writer.Close()
	return body.Bytes()
}

func First(a interface{}, b interface{}) interface{} {
	return a
}

func Second(a interface{}, b interface{}) interface{} {
	return b
}
