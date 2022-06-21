package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"github.com/gin-gonic/gin"
	"github.com/golang/mock/gomock"
	"github.com/joschahenningsen/TUM-Live/dao"
	"github.com/joschahenningsen/TUM-Live/mock_dao"
	"github.com/joschahenningsen/TUM-Live/model"
	"github.com/joschahenningsen/TUM-Live/tools"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestProgressReport(t *testing.T) {
	const PROGRESS_REPORT_URL = "/api/progressReport"

	t.Run("POST [invalid body]", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, r := gin.CreateTestContext(w)

		configProgressRouter(r, dao.DaoWrapper{})

		c.Request, _ = http.NewRequest(http.MethodPost, PROGRESS_REPORT_URL, nil)
		r.ServeHTTP(w, c.Request)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("POST [no context]", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, r := gin.CreateTestContext(w)

		body, _ := json.Marshal(progressRequest{
			StreamID: uint(1),
			Progress: 0,
		})

		configProgressRouter(r, dao.DaoWrapper{})

		c.Request, _ = http.NewRequest(http.MethodPost, PROGRESS_REPORT_URL, bytes.NewBuffer(body))
		r.ServeHTTP(w, c.Request)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("POST [not logged in]", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, r := gin.CreateTestContext(w)

		body, _ := json.Marshal(progressRequest{
			StreamID: uint(1),
			Progress: 0,
		})

		r.Use(func(c *gin.Context) {
			c.Set("TUMLiveContext", tools.TUMLiveContext{User: nil})
		})

		configProgressRouter(r, dao.DaoWrapper{})

		c.Request, _ = http.NewRequest(http.MethodPost, PROGRESS_REPORT_URL, bytes.NewBuffer(body))
		r.ServeHTTP(w, c.Request)

		assert.Equal(t, http.StatusForbidden, w.Code)
	})

	t.Run("POST [success]", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, r := gin.CreateTestContext(w)

		body, _ := json.Marshal(progressRequest{
			StreamID: uint(1),
			Progress: 0,
		})

		r.Use(func(c *gin.Context) {
			c.Set("TUMLiveContext", tools.TUMLiveContext{User: &model.User{}})
		})

		configProgressRouter(r, dao.DaoWrapper{})

		c.Request, _ = http.NewRequest(http.MethodPost, PROGRESS_REPORT_URL, bytes.NewBuffer(body))
		r.ServeHTTP(w, c.Request)

		assert.Equal(t, http.StatusOK, w.Code)
	})
}

func TestWatched(t *testing.T) {
	const WATCHED_URL = "/api/watched"

	t.Run("POST [invalid body]", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, r := gin.CreateTestContext(w)

		configProgressRouter(r, dao.DaoWrapper{})

		c.Request, _ = http.NewRequest(http.MethodPost, WATCHED_URL, nil)
		r.ServeHTTP(w, c.Request)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("POST [no context]", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, r := gin.CreateTestContext(w)

		body, _ := json.Marshal(watchedRequest{
			StreamID: uint(1),
			Watched:  true,
		})

		configProgressRouter(r, dao.DaoWrapper{})

		c.Request, _ = http.NewRequest(http.MethodPost, WATCHED_URL, bytes.NewBuffer(body))
		r.ServeHTTP(w, c.Request)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("POST [not logged in]", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, r := gin.CreateTestContext(w)

		body, _ := json.Marshal(watchedRequest{
			StreamID: uint(1),
			Watched:  true,
		})

		r.Use(func(c *gin.Context) {
			c.Set("TUMLiveContext", tools.TUMLiveContext{User: nil})
		})

		configProgressRouter(r, dao.DaoWrapper{})

		c.Request, _ = http.NewRequest(http.MethodPost, WATCHED_URL, bytes.NewBuffer(body))
		r.ServeHTTP(w, c.Request)

		assert.Equal(t, http.StatusForbidden, w.Code)
	})

	t.Run("POST [SaveProgresses returns error]", func(t *testing.T) {
		progressMock := mock_dao.NewMockProgressDao(gomock.NewController(t))

		w := httptest.NewRecorder()
		c, r := gin.CreateTestContext(w)

		body, _ := json.Marshal(watchedRequest{
			StreamID: uint(1),
			Watched:  true,
		})

		r.Use(func(c *gin.Context) {
			c.Set("TUMLiveContext", tools.TUMLiveContext{User: &model.User{}})
		})

		progressMock.EXPECT().SaveProgresses(gomock.Any()).Return(errors.New(""))

		configProgressRouter(r, dao.DaoWrapper{ProgressDao: progressMock})

		c.Request, _ = http.NewRequest(http.MethodPost, WATCHED_URL, bytes.NewBuffer(body))
		r.ServeHTTP(w, c.Request)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})

	t.Run("POST [success]", func(t *testing.T) {
		progressMock := mock_dao.NewMockProgressDao(gomock.NewController(t))

		w := httptest.NewRecorder()
		c, r := gin.CreateTestContext(w)

		body, _ := json.Marshal(watchedRequest{
			StreamID: uint(1),
			Watched:  true,
		})

		r.Use(func(c *gin.Context) {
			c.Set("TUMLiveContext", tools.TUMLiveContext{User: &model.User{
				Model: gorm.Model{ID: 1},
			}})
		})

		progressMock.EXPECT().SaveProgresses(gomock.Any()).Return(nil)

		configProgressRouter(r, dao.DaoWrapper{ProgressDao: progressMock})

		c.Request, _ = http.NewRequest(http.MethodPost, WATCHED_URL, bytes.NewBuffer(body))
		r.ServeHTTP(w, c.Request)

		assert.Equal(t, http.StatusOK, w.Code)
	})
}
