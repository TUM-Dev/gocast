package api

import (
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
	"gorm.io/gorm"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestStatistics(t *testing.T) {
	gin.SetMode(gin.TestMode)

	addAdminContext := func(r *gin.Engine, courseId uint) {
		r.Use(func(c *gin.Context) {
			c.Set("TUMLiveContext", tools.TUMLiveContext{User: &model.User{
				Role: model.AdminType,
				AdministeredCourses: []model.Course{
					{Model: gorm.Model{ID: courseId}},
				},
			}})
		})
	}

	t.Run("GET[Invalid Body]", func(t *testing.T) {
		coursesMock := mock_dao.NewMockCoursesDao(gomock.NewController(t))

		courseId := uint(1)

		w := httptest.NewRecorder()
		c, r := gin.CreateTestContext(w)

		addAdminContext(r, courseId)

		coursesMock.
			EXPECT().
			GetCourseById(gomock.Any(), courseId).
			Return(model.Course{}, nil).
			AnyTimes()

		configGinCourseRouter(r, dao.DaoWrapper{CoursesDao: coursesMock})

		c.Request, _ = http.NewRequest(http.MethodGet,
			fmt.Sprintf("/api/course/%d/stats", courseId), nil)
		r.ServeHTTP(w, c.Request)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("GET[cid==0 - not admin]", func(t *testing.T) {
		coursesMock := mock_dao.NewMockCoursesDao(gomock.NewController(t))

		courseId := uint(0)

		w := httptest.NewRecorder()
		c, r := gin.CreateTestContext(w)

		r.Use(func(c *gin.Context) {
			c.Set("TUMLiveContext", tools.TUMLiveContext{User: &model.User{
				Role: model.StudentType,
				AdministeredCourses: []model.Course{
					{Model: gorm.Model{ID: courseId}},
				},
			}})
		})

		coursesMock.
			EXPECT().
			GetCourseById(gomock.Any(), courseId).
			Return(model.Course{}, nil).
			AnyTimes()

		configGinCourseRouter(r, dao.DaoWrapper{CoursesDao: coursesMock})
		c.Request, _ = http.NewRequest(http.MethodGet,
			fmt.Sprintf("/api/course/%d/stats?interval=week", courseId), nil)
		r.ServeHTTP(w, c.Request)

		assert.Equal(t, http.StatusForbidden, w.Code)
	})

	t.Run("GET[invalid interval]", func(t *testing.T) {
		coursesMock := mock_dao.NewMockCoursesDao(gomock.NewController(t))

		courseId := uint(1)

		coursesMock.
			EXPECT().
			GetCourseById(gomock.Any(), courseId).
			Return(model.Course{Model: gorm.Model{ID: courseId}}, nil).
			AnyTimes()

		w := httptest.NewRecorder()
		c, r := gin.CreateTestContext(w)

		addAdminContext(r, courseId)

		configGinCourseRouter(r, dao.DaoWrapper{CoursesDao: coursesMock})

		c.Request, _ = http.NewRequest(http.MethodGet,
			fmt.Sprintf("/api/course/%d/stats?interval=1234", courseId), nil)
		r.ServeHTTP(w, c.Request)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("GET[DAO functions return error]", func(t *testing.T) {
		coursesMock := mock_dao.NewMockCoursesDao(gomock.NewController(t))
		statisticsMock := mock_dao.NewMockStatisticsDao(gomock.NewController(t))

		courseId := uint(1)

		intervals := []string{"week", "day", "hour", "activity-live", "activity-vod", "numStudents", "vodViews", "liveViews", "allDays"}

		statisticsMock.
			EXPECT().
			GetCourseStatsWeekdays(courseId).
			Return([]dao.Stat{}, errors.New("")).
			AnyTimes()

		statisticsMock.
			EXPECT().
			GetCourseStatsHourly(courseId).
			Return([]dao.Stat{}, errors.New("")).
			AnyTimes()

		statisticsMock.
			EXPECT().
			GetStudentActivityCourseStats(courseId, true).
			Return([]dao.Stat{}, errors.New("")).
			AnyTimes()

		statisticsMock.
			EXPECT().
			GetStudentActivityCourseStats(courseId, false).
			Return([]dao.Stat{}, errors.New("")).
			AnyTimes()

		statisticsMock.
			EXPECT().
			GetCourseNumStudents(courseId).
			Return(int64(0), errors.New("")).
			AnyTimes()

		statisticsMock.
			EXPECT().
			GetCourseNumVodViews(courseId).
			Return(0, errors.New("")).
			AnyTimes()

		statisticsMock.
			EXPECT().
			GetCourseNumLiveViews(courseId).
			Return(0, errors.New("")).
			AnyTimes()

		statisticsMock.
			EXPECT().
			GetCourseNumVodViewsPerDay(courseId).
			Return([]dao.Stat{}, errors.New("")).
			AnyTimes()

		coursesMock.
			EXPECT().
			GetCourseById(gomock.Any(), courseId).
			Return(model.Course{Model: gorm.Model{ID: courseId}}, nil).
			AnyTimes()

		for _, interval := range intervals {
			w := httptest.NewRecorder()
			c, r := gin.CreateTestContext(w)

			addAdminContext(r, courseId)

			configGinCourseRouter(r, dao.DaoWrapper{CoursesDao: coursesMock, StatisticsDao: statisticsMock})

			c.Request, _ = http.NewRequest(http.MethodGet,
				fmt.Sprintf("/api/course/%d/stats?interval=%s", courseId, interval), nil)
			r.ServeHTTP(w, c.Request)

			assert.Equal(t, http.StatusInternalServerError, w.Code)
		}
	})

	t.Run("GET[success]", func(t *testing.T) {
		coursesMock := mock_dao.NewMockCoursesDao(gomock.NewController(t))
		statisticsMock := mock_dao.NewMockStatisticsDao(gomock.NewController(t))

		courseId := uint(1)

		stats := []dao.Stat{
			{X: "1", Y: 10},
			{X: "2", Y: 361},
			{X: "3", Y: 144},
		}

		resp := chartJs{
			Data:    chartJsData{Datasets: []chartJsDataset{newChartJsDataset()}},
			Options: newChartJsOptions(),
		}

		coursesMock.
			EXPECT().
			GetCourseById(gomock.Any(), courseId).
			Return(model.Course{Model: gorm.Model{ID: courseId}}, nil).
			AnyTimes()

		t.Run("interval week or day", func(t *testing.T) {
			resp.ChartType = "bar"
			resp.Data.Datasets[0].Label = "Sum(viewers)"
			resp.Data.Datasets[0].Data = stats

			respJson, _ := json.Marshal(resp)

			w := httptest.NewRecorder()
			c, r := gin.CreateTestContext(w)

			addAdminContext(r, courseId)

			statisticsMock.
				EXPECT().
				GetCourseStatsWeekdays(courseId).
				Return(stats, nil).
				AnyTimes()

			configGinCourseRouter(r, dao.DaoWrapper{
				CoursesDao:    coursesMock,
				StatisticsDao: statisticsMock})

			c.Request, _ = http.NewRequest(http.MethodGet,
				fmt.Sprintf("/api/course/%d/stats?interval=week", courseId), nil)
			r.ServeHTTP(w, c.Request)

			assert.Equal(t, http.StatusOK, w.Code)
			assert.Equal(t, string(respJson), w.Body.String())
		})

		t.Run("interval week or day, cid==0 - admin", func(t *testing.T) {
			courseId = 0
			resp.ChartType = "bar"
			resp.Data.Datasets[0].Label = "Sum(viewers)"
			resp.Data.Datasets[0].Data = stats

			respJson, _ := json.Marshal(resp)

			w := httptest.NewRecorder()
			c, r := gin.CreateTestContext(w)

			addAdminContext(r, courseId)

			// re-mock GetCourseById since courseId has to be 0
			coursesMock.
				EXPECT().
				GetCourseById(gomock.Any(), courseId).
				Return(model.Course{Model: gorm.Model{ID: courseId}}, nil).
				AnyTimes()

			statisticsMock.
				EXPECT().
				GetCourseStatsWeekdays(courseId).
				Return(stats, nil).
				AnyTimes()

			configGinCourseRouter(r, dao.DaoWrapper{
				CoursesDao:    coursesMock,
				StatisticsDao: statisticsMock})

			c.Request, _ = http.NewRequest(http.MethodGet,
				fmt.Sprintf("/api/course/%d/stats?interval=week", courseId), nil)
			r.ServeHTTP(w, c.Request)

			assert.Equal(t, http.StatusOK, w.Code)
			assert.Equal(t, string(respJson), w.Body.String())
		})

		t.Run("interval hour", func(t *testing.T) {
			resp.ChartType = "bar"
			resp.Data.Datasets[0].Label = "Sum(viewers)"
			resp.Data.Datasets[0].Data = stats

			respJson, _ := json.Marshal(resp)

			w := httptest.NewRecorder()
			c, r := gin.CreateTestContext(w)

			addAdminContext(r, courseId)

			statisticsMock.
				EXPECT().
				GetCourseStatsHourly(courseId).
				Return(stats, nil).
				AnyTimes()

			configGinCourseRouter(r, dao.DaoWrapper{
				CoursesDao:    coursesMock,
				StatisticsDao: statisticsMock})

			c.Request, _ = http.NewRequest(http.MethodGet,
				fmt.Sprintf("/api/course/%d/stats?interval=hour", courseId), nil)
			r.ServeHTTP(w, c.Request)

			assert.Equal(t, http.StatusOK, w.Code)
			assert.Equal(t, string(respJson), w.Body.String())
		})

		t.Run("interval activity-live", func(t *testing.T) {
			resp.ChartType = "line"
			resp.Data.Datasets[0].Label = "Live"
			resp.Data.Datasets[0].Data = stats
			resp.Data.Datasets[0].BorderColor = "#d12a5c"
			resp.Data.Datasets[0].BackgroundColor = ""

			respJson, _ := json.Marshal(resp)

			w := httptest.NewRecorder()
			c, r := gin.CreateTestContext(w)

			addAdminContext(r, courseId)

			statisticsMock.
				EXPECT().
				GetStudentActivityCourseStats(courseId, true).
				Return(stats, nil).
				AnyTimes()

			configGinCourseRouter(r, dao.DaoWrapper{
				CoursesDao:    coursesMock,
				StatisticsDao: statisticsMock})

			c.Request, _ = http.NewRequest(http.MethodGet,
				fmt.Sprintf("/api/course/%d/stats?interval=activity-live", courseId), nil)
			r.ServeHTTP(w, c.Request)

			assert.Equal(t, http.StatusOK, w.Code)
			assert.Equal(t, string(respJson), w.Body.String())
		})

		t.Run("interval activity-vod", func(t *testing.T) {
			resp.ChartType = "line"
			resp.Data.Datasets[0].Label = "VoD"
			resp.Data.Datasets[0].Data = stats
			resp.Data.Datasets[0].BorderColor = "#2a7dd1"
			resp.Data.Datasets[0].BackgroundColor = ""

			respJson, _ := json.Marshal(resp)

			w := httptest.NewRecorder()
			c, r := gin.CreateTestContext(w)

			addAdminContext(r, courseId)

			statisticsMock.
				EXPECT().
				GetStudentActivityCourseStats(courseId, false).
				Return(stats, nil).
				AnyTimes()

			configGinCourseRouter(r, dao.DaoWrapper{
				CoursesDao:    coursesMock,
				StatisticsDao: statisticsMock})

			c.Request, _ = http.NewRequest(http.MethodGet,
				fmt.Sprintf("/api/course/%d/stats?interval=activity-vod", courseId), nil)
			r.ServeHTTP(w, c.Request)

			assert.Equal(t, http.StatusOK, w.Code)
			assert.Equal(t, string(respJson), w.Body.String())
		})

		t.Run("interval numStudents", func(t *testing.T) {
			numStudents := int64(1337)

			respJson, _ := json.Marshal(gin.H{"res": numStudents})

			w := httptest.NewRecorder()
			c, r := gin.CreateTestContext(w)

			addAdminContext(r, courseId)

			statisticsMock.
				EXPECT().
				GetCourseNumStudents(courseId).
				Return(numStudents, nil).
				AnyTimes()

			configGinCourseRouter(r, dao.DaoWrapper{
				CoursesDao:    coursesMock,
				StatisticsDao: statisticsMock})

			c.Request, _ = http.NewRequest(http.MethodGet,
				fmt.Sprintf("/api/course/%d/stats?interval=numStudents", courseId), nil)
			r.ServeHTTP(w, c.Request)

			assert.Equal(t, http.StatusOK, w.Code)
			assert.Equal(t, string(respJson), w.Body.String())
		})

		t.Run("interval vodViews", func(t *testing.T) {
			views := 1001

			respJson, _ := json.Marshal(gin.H{"res": views})

			w := httptest.NewRecorder()
			c, r := gin.CreateTestContext(w)

			addAdminContext(r, courseId)

			statisticsMock.
				EXPECT().
				GetCourseNumVodViews(courseId).
				Return(views, nil).
				AnyTimes()

			configGinCourseRouter(r, dao.DaoWrapper{
				CoursesDao:    coursesMock,
				StatisticsDao: statisticsMock})

			c.Request, _ = http.NewRequest(http.MethodGet,
				fmt.Sprintf("/api/course/%d/stats?interval=vodViews", courseId), nil)
			r.ServeHTTP(w, c.Request)

			assert.Equal(t, http.StatusOK, w.Code)
			assert.Equal(t, string(respJson), w.Body.String())
		})

		t.Run("interval liveViews", func(t *testing.T) {
			views := 101

			respJson, _ := json.Marshal(gin.H{"res": views})

			w := httptest.NewRecorder()
			c, r := gin.CreateTestContext(w)

			addAdminContext(r, courseId)

			statisticsMock.
				EXPECT().
				GetCourseNumLiveViews(courseId).
				Return(views, nil).
				AnyTimes()

			configGinCourseRouter(r, dao.DaoWrapper{
				CoursesDao:    coursesMock,
				StatisticsDao: statisticsMock})

			c.Request, _ = http.NewRequest(http.MethodGet,
				fmt.Sprintf("/api/course/%d/stats?interval=liveViews", courseId), nil)
			r.ServeHTTP(w, c.Request)

			assert.Equal(t, http.StatusOK, w.Code)
			assert.Equal(t, string(respJson), w.Body.String())
		})

		t.Run("interval allDays", func(t *testing.T) {
			resp.ChartType = "bar"
			resp.Data.Datasets[0].Label = "views"
			resp.Data.Datasets[0].Data = stats
			resp.Data.Datasets[0].BorderColor = "#427dbd"
			resp.Data.Datasets[0].BackgroundColor = "#d12a5c"

			respJson, _ := json.Marshal(resp)

			w := httptest.NewRecorder()
			c, r := gin.CreateTestContext(w)

			addAdminContext(r, courseId)

			statisticsMock.
				EXPECT().
				GetCourseNumVodViewsPerDay(courseId).
				Return(stats, nil).
				AnyTimes()

			configGinCourseRouter(r, dao.DaoWrapper{
				CoursesDao:    coursesMock,
				StatisticsDao: statisticsMock})

			c.Request, _ = http.NewRequest(http.MethodGet,
				fmt.Sprintf("/api/course/%d/stats?interval=allDays", courseId), nil)
			r.ServeHTTP(w, c.Request)

			assert.Equal(t, http.StatusOK, w.Code)
			assert.Equal(t, string(respJson), w.Body.String())
		})
	})
}
