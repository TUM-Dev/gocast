package testutils

import (
	"bytes"
	"encoding/json"
	"errors"
	"github.com/gin-gonic/gin"
	"github.com/joschahenningsen/TUM-Live/dao"
	"github.com/joschahenningsen/TUM-Live/tools"
	"github.com/stretchr/testify/assert"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

type TestCases map[string]TestCase

type TestCase struct {
	Method           string
	Url              string
	DaoWrapper       dao.DaoWrapper
	TumLiveContext   *tools.TUMLiveContext
	ContentType      string
	Body             interface{}
	ExpectedCode     int
	ExpectedResponse []byte

	Before func()
}

func (tc TestCases) Run(t *testing.T, configRouterFunc func(*gin.Engine, dao.DaoWrapper)) {
	for name, testCase := range tc {
		if testCase.Before != nil {
			testCase.Before()
		}

		// Run test
		t.Run(name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, r := gin.CreateTestContext(w)

			if testCase.TumLiveContext != nil {
				r.Use(func(c *gin.Context) {
					c.Set("TUMLiveContext", *testCase.TumLiveContext)
				})
			}

			configRouterFunc(r, testCase.DaoWrapper)

			c.Request, _ = http.NewRequest(testCase.Method, testCase.Url, testCase.getBody())
			c.Request.Header.Set("Content-Type", testCase.getContentType())
			r.ServeHTTP(w, c.Request)

			assert.Equal(t, testCase.ExpectedCode, w.Code)

			if len(testCase.ExpectedResponse) > 0 {
				assert.Equal(t, string(testCase.ExpectedResponse), w.Body.String())
			}
		})
	}
}

func (c TestCase) getContentType() string {
	if len(c.ContentType) > 0 {
		return c.ContentType
	}

	return "application/json"
}

func (c TestCase) getBody() io.Reader {
	if c.Body == nil {
		return bytes.NewBuffer([]byte{})
	}
	switch c.Body.(type) {
	case io.Reader:
		return c.Body.(io.Reader)
	case string:
		return bytes.NewBufferString(c.Body.(string))
	case []byte:
		return bytes.NewBuffer(c.Body.([]byte))
	default:
		j, err := json.Marshal(c.Body)
		if err != nil {
			panic(errors.New("invalid body type"))
		}

		return bytes.NewBuffer(j)
	}
}

func NewMultipartFormData(fieldName, fileName string) (bytes.Buffer, *multipart.Writer) {
	var b bytes.Buffer
	w := multipart.NewWriter(&b)
	file, _ := os.Open(fileName)
	fw, _ := w.CreateFormFile(fieldName, file.Name())
	io.Copy(fw, file)
	w.Close()
	return b, w
}

func First(a interface{}, b interface{}) interface{} {
	return a
}

func Second(a interface{}, b interface{}) interface{} {
	return b
}
