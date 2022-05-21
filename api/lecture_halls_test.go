package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/golang/mock/gomock"
	"github.com/joschahenningsen/TUM-Live/dao"
	"github.com/joschahenningsen/TUM-Live/mock_dao"
	"github.com/joschahenningsen/TUM-Live/model"
	"github.com/joschahenningsen/TUM-Live/tools"
	"github.com/stretchr/testify/assert"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLectureHallsCRUD(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("POST[Invalid Body]", func(t *testing.T) {
		lectureHallId := uint(1)

		w := httptest.NewRecorder()
		c, r := gin.CreateTestContext(w)

		r.Use(func(c *gin.Context) {
			c.Set("TUMLiveContext", tools.TUMLiveContext{User: &model.User{
				Role: model.AdminType,
			}})
		})

		lectureHallMock := mock_dao.NewMockLectureHallsDao(gomock.NewController(t))
		lectureHallMock.
			EXPECT().
			DeleteLectureHall(lectureHallId).
			Return(errors.New("")).
			AnyTimes()

		configGinLectureHallApiRouter(r, dao.DaoWrapper{LectureHallsDao: lectureHallMock})

		c.Request, _ = http.NewRequest(http.MethodPost, "/api/createLectureHall", nil)
		r.ServeHTTP(w, c.Request)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("POST[Success]", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, r := gin.CreateTestContext(w)

		r.Use(func(c *gin.Context) {
			c.Set("TUMLiveContext", tools.TUMLiveContext{User: &model.User{
				Role: model.AdminType,
			}})
		})

		body, _ := json.Marshal(createLectureHallRequest{
			Name:      "LH1",
			CombIP:    "0.0.0.0",
			PresIP:    "0.0.0.0",
			CamIP:     "0.0.0.0",
			CameraIP:  "0.0.0.0",
			PwrCtrlIP: "0.0.0.0",
		})

		lectureHallMock := mock_dao.NewMockLectureHallsDao(gomock.NewController(t))
		lectureHallMock.
			EXPECT().
			CreateLectureHall(gomock.Any()).AnyTimes()

		configGinLectureHallApiRouter(r, dao.DaoWrapper{LectureHallsDao: lectureHallMock})

		c.Request, _ = http.NewRequest(http.MethodPost, "/api/createLectureHall", bytes.NewBuffer(body))
		r.ServeHTTP(w, c.Request)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("PUT[Invalid Body]", func(t *testing.T) {
		lectureHallId := uint(1)

		w := httptest.NewRecorder()
		c, r := gin.CreateTestContext(w)

		r.Use(func(c *gin.Context) {
			c.Set("TUMLiveContext", tools.TUMLiveContext{User: &model.User{
				Role: model.AdminType,
			}})
		})

		configGinLectureHallApiRouter(r, dao.DaoWrapper{})

		c.Request, _ = http.NewRequest(http.MethodPut,
			fmt.Sprintf("/api/lectureHall/%d", lectureHallId), nil)
		r.ServeHTTP(w, c.Request)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("PUT[id not integer]", func(t *testing.T) {
		lectureHallId := "abc"

		w := httptest.NewRecorder()
		c, r := gin.CreateTestContext(w)

		r.Use(func(c *gin.Context) {
			c.Set("TUMLiveContext", tools.TUMLiveContext{User: &model.User{
				Role: model.AdminType,
			}})
		})

		configGinLectureHallApiRouter(r, dao.DaoWrapper{})

		jBody, _ := json.Marshal(updateLectureHallReq{CamIp: "0.0.0.0"})

		c.Request, _ = http.NewRequest(http.MethodPut,
			fmt.Sprintf("/api/lectureHall/%s", lectureHallId), bytes.NewBuffer(jBody))
		r.ServeHTTP(w, c.Request)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("PUT[GetLectureHallByID returns error]", func(t *testing.T) {
		lectureHallId := uint(1)

		w := httptest.NewRecorder()
		c, r := gin.CreateTestContext(w)

		r.Use(func(c *gin.Context) {
			c.Set("TUMLiveContext", tools.TUMLiveContext{User: &model.User{
				Role: model.AdminType,
			}})
		})

		lectureHallMock := mock_dao.NewMockLectureHallsDao(gomock.NewController(t))
		lectureHallMock.
			EXPECT().
			GetLectureHallByID(lectureHallId).
			Return(model.LectureHall{},
				errors.New("")).
			AnyTimes()

		configGinLectureHallApiRouter(r, dao.DaoWrapper{LectureHallsDao: lectureHallMock})

		jBody, _ := json.Marshal(updateLectureHallReq{CamIp: "0.0.0.0"})

		c.Request, _ = http.NewRequest(http.MethodPut,
			fmt.Sprintf("/api/lectureHall/%d", lectureHallId), bytes.NewBuffer(jBody))
		r.ServeHTTP(w, c.Request)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("PUT[SaveLectureHall returns error]", func(t *testing.T) {
		lectureHallId := uint(1)

		w := httptest.NewRecorder()
		c, r := gin.CreateTestContext(w)

		r.Use(func(c *gin.Context) {
			c.Set("TUMLiveContext", tools.TUMLiveContext{User: &model.User{
				Role: model.AdminType,
			}})
		})

		lectureHallMock := mock_dao.NewMockLectureHallsDao(gomock.NewController(t))
		lectureHallMock.
			EXPECT().
			GetLectureHallByID(lectureHallId).
			Return(model.LectureHall{}, nil).
			AnyTimes()
		lectureHallMock.
			EXPECT().
			SaveLectureHall(gomock.Any()).
			Return(errors.New("")).
			AnyTimes()

		configGinLectureHallApiRouter(r, dao.DaoWrapper{LectureHallsDao: lectureHallMock})

		jBody, _ := json.Marshal(updateLectureHallReq{CamIp: "0.0.0.0"})

		c.Request, _ = http.NewRequest(http.MethodPut,
			fmt.Sprintf("/api/lectureHall/%d", lectureHallId), bytes.NewBuffer(jBody))
		r.ServeHTTP(w, c.Request)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})

	t.Run("PUT[success]", func(t *testing.T) {
		lectureHallId := uint(1)

		w := httptest.NewRecorder()
		c, r := gin.CreateTestContext(w)

		r.Use(func(c *gin.Context) {
			c.Set("TUMLiveContext", tools.TUMLiveContext{User: &model.User{
				Role: model.AdminType,
			}})
		})

		lectureHallMock := mock_dao.NewMockLectureHallsDao(gomock.NewController(t))
		lectureHallMock.
			EXPECT().
			GetLectureHallByID(lectureHallId).
			Return(model.LectureHall{}, nil)
		lectureHallMock.
			EXPECT().
			SaveLectureHall(gomock.Any()).
			Return(nil)

		configGinLectureHallApiRouter(r, dao.DaoWrapper{LectureHallsDao: lectureHallMock})

		jBody, _ := json.Marshal(updateLectureHallReq{CamIp: "0.0.0.0"})

		c.Request, _ = http.NewRequest(http.MethodPut,
			fmt.Sprintf("/api/lectureHall/%d", lectureHallId), bytes.NewBuffer(jBody))
		r.ServeHTTP(w, c.Request)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("DELETE[id not parameter]", func(t *testing.T) {
		lectureHallId := "abc"

		w := httptest.NewRecorder()
		c, r := gin.CreateTestContext(w)

		r.Use(func(c *gin.Context) {
			c.Set("TUMLiveContext", tools.TUMLiveContext{User: &model.User{
				Role: model.AdminType,
			}})
		})

		configGinLectureHallApiRouter(r, dao.DaoWrapper{})

		c.Request, _ = http.NewRequest(http.MethodDelete,
			fmt.Sprintf("/api/lectureHall/%s", lectureHallId), nil)
		r.ServeHTTP(w, c.Request)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("DELETE[DeleteLectureHall returns error]", func(t *testing.T) {
		lectureHallId := uint(1)

		w := httptest.NewRecorder()
		c, r := gin.CreateTestContext(w)

		r.Use(func(c *gin.Context) {
			c.Set("TUMLiveContext", tools.TUMLiveContext{User: &model.User{
				Role: model.AdminType,
			}})
		})

		lectureHallMock := mock_dao.NewMockLectureHallsDao(gomock.NewController(t))
		lectureHallMock.
			EXPECT().
			DeleteLectureHall(lectureHallId).
			Return(errors.New("")).
			AnyTimes()

		configGinLectureHallApiRouter(r, dao.DaoWrapper{LectureHallsDao: lectureHallMock})

		c.Request, _ = http.NewRequest(http.MethodDelete,
			fmt.Sprintf("/api/lectureHall/%d", lectureHallId), nil)
		r.ServeHTTP(w, c.Request)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})

	t.Run("DELETE[success]", func(t *testing.T) {
		lectureHallId := uint(1)

		w := httptest.NewRecorder()
		c, r := gin.CreateTestContext(w)

		r.Use(func(c *gin.Context) {
			c.Set("TUMLiveContext", tools.TUMLiveContext{User: &model.User{
				Role: model.AdminType,
			}})
		})

		lectureHallMock := mock_dao.NewMockLectureHallsDao(gomock.NewController(t))
		lectureHallMock.
			EXPECT().
			DeleteLectureHall(lectureHallId).
			Return(nil).
			AnyTimes()

		configGinLectureHallApiRouter(r, dao.DaoWrapper{LectureHallsDao: lectureHallMock})

		c.Request, _ = http.NewRequest(http.MethodDelete,
			fmt.Sprintf("/api/lectureHall/%d", lectureHallId), nil)
		r.ServeHTTP(w, c.Request)

		assert.Equal(t, http.StatusOK, w.Code)
	})
}
