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
	"github.com/joschahenningsen/TUM-Live/tools/testutils"
	"gorm.io/gorm"
	"net/http"
	"testing"
)

func TestStatistics(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("GET/api/course/:courseID/stats", func(t *testing.T) {
		baseUrl := fmt.Sprintf("/api/course/%d/stats", testutils.CourseFPV.ID)

		var res []byte

		stats := []dao.Stat{
			{X: "1", Y: 10},
			{X: "2", Y: 361},
			{X: "3", Y: 144},
		}

		resp := chartJs{
			Data:    chartJsData{Datasets: []chartJsDataset{newChartJsDataset()}},
			Options: newChartJsOptions(),
		}

		numStudents := int64(1337)
		views := 1001

		intervals := []string{"week", "day", "hour", "activity-live", "activity-vod", "numStudents", "vodViews", "liveViews", "allDays"}

		testCases := testutils.TestCases{
			"invalid body": {
				Method:         http.MethodGet,
				Url:            baseUrl,
				TumLiveContext: &testutils.TUMLiveContextAdmin,
				DaoWrapper: dao.DaoWrapper{
					CoursesDao: testutils.GetCoursesMock(t),
				},
				ExpectedCode: http.StatusBadRequest,
			},
			"courseID 0, not admin": {
				Method:         http.MethodGet,
				Url:            fmt.Sprintf("/api/course/%d/stats%s", 0, "?interval=week"),
				TumLiveContext: &testutils.TUMLiveContextStudent,
				DaoWrapper: dao.DaoWrapper{
					CoursesDao: func() dao.CoursesDao {
						coursesMock := mock_dao.NewMockCoursesDao(gomock.NewController(t))
						coursesMock.
							EXPECT().
							GetCourseById(gomock.Any(), uint(0)).
							Return(testutils.CourseFPV, nil).
							AnyTimes()
						return coursesMock
					}(),
				},
				ExpectedCode: http.StatusForbidden,
			},
			"invalid interval": {
				Method:         http.MethodGet,
				Url:            fmt.Sprintf("%s?interval=%s", baseUrl, "century"),
				TumLiveContext: &testutils.TUMLiveContextAdmin,
				DaoWrapper: dao.DaoWrapper{
					CoursesDao: testutils.GetCoursesMock(t),
				},
				ExpectedCode: http.StatusBadRequest,
			},
			"success week,day": {
				Method:         http.MethodGet,
				Url:            fmt.Sprintf("%s?interval=week", baseUrl),
				TumLiveContext: &testutils.TUMLiveContextAdmin,
				DaoWrapper: dao.DaoWrapper{
					CoursesDao: testutils.GetCoursesMock(t),
					StatisticsDao: func() dao.StatisticsDao {
						statisticsMock := mock_dao.NewMockStatisticsDao(gomock.NewController(t))
						statisticsMock.
							EXPECT().
							GetCourseStatsWeekdays(testutils.CourseFPV.ID).
							Return(stats, nil).
							AnyTimes()
						return statisticsMock
					}(),
				},
				Before: func() {
					resp.ChartType = "bar"
					resp.Data.Datasets[0].Label = "Sum(viewers)"
					resp.Data.Datasets[0].Data = stats

					res, _ = json.Marshal(resp)
				},
				ExpectedCode:     http.StatusOK,
				ExpectedResponse: res,
			},
			"success courseID 0, week,day": {
				Method:         http.MethodGet,
				Url:            fmt.Sprintf("/api/course/%d/stats%s", 0, "?interval=week"),
				TumLiveContext: &testutils.TUMLiveContextAdmin,
				DaoWrapper: dao.DaoWrapper{
					CoursesDao: func() dao.CoursesDao {
						coursesMock := mock_dao.NewMockCoursesDao(gomock.NewController(t))
						coursesMock.
							EXPECT().
							GetCourseById(gomock.Any(), uint(0)).
							Return(model.Course{Model: gorm.Model{ID: uint(0)}}, nil).
							AnyTimes()
						return coursesMock
					}(),
					StatisticsDao: func() dao.StatisticsDao {
						statisticsMock := mock_dao.NewMockStatisticsDao(gomock.NewController(t))
						statisticsMock.
							EXPECT().
							GetCourseStatsWeekdays(uint(0)).
							Return(stats, nil).
							AnyTimes()
						return statisticsMock
					}(),
				},
				Before: func() {
					resp.ChartType = "bar"
					resp.Data.Datasets[0].Label = "Sum(viewers)"
					resp.Data.Datasets[0].Data = stats

					res, _ = json.Marshal(resp)
				},
				ExpectedCode:     http.StatusOK,
				ExpectedResponse: res,
			},
			"success hour": {
				Method:         http.MethodGet,
				Url:            fmt.Sprintf("%s?interval=hour", baseUrl),
				TumLiveContext: &testutils.TUMLiveContextAdmin,
				DaoWrapper: dao.DaoWrapper{
					CoursesDao: testutils.GetCoursesMock(t),
					StatisticsDao: func() dao.StatisticsDao {
						statisticsMock := mock_dao.NewMockStatisticsDao(gomock.NewController(t))
						statisticsMock.
							EXPECT().
							GetCourseStatsHourly(testutils.CourseFPV.ID).
							Return(stats, nil).
							AnyTimes()
						return statisticsMock
					}(),
				},
				Before: func() {
					resp.ChartType = "bar"
					resp.Data.Datasets[0].Label = "Sum(viewers)"
					resp.Data.Datasets[0].Data = stats

					res, _ = json.Marshal(resp)
				},
				ExpectedCode:     http.StatusOK,
				ExpectedResponse: res,
			},
			"success activity-live": {
				Method:         http.MethodGet,
				Url:            fmt.Sprintf("%s?interval=activity-live", baseUrl),
				TumLiveContext: &testutils.TUMLiveContextAdmin,
				DaoWrapper: dao.DaoWrapper{
					CoursesDao: testutils.GetCoursesMock(t),
					StatisticsDao: func() dao.StatisticsDao {
						statisticsMock := mock_dao.NewMockStatisticsDao(gomock.NewController(t))
						statisticsMock.
							EXPECT().
							GetStudentActivityCourseStats(testutils.CourseFPV.ID, true).
							Return(stats, nil).
							AnyTimes()
						return statisticsMock
					}(),
				},
				Before: func() {
					resp.ChartType = "line"
					resp.Data.Datasets[0].Label = "Live"
					resp.Data.Datasets[0].Data = stats
					resp.Data.Datasets[0].BorderColor = "#d12a5c"
					resp.Data.Datasets[0].BackgroundColor = ""

					res, _ = json.Marshal(resp)
				},
				ExpectedCode:     http.StatusOK,
				ExpectedResponse: res,
			},
			"success activity-vod": {
				Method:         http.MethodGet,
				Url:            fmt.Sprintf("%s?interval=activity-vod", baseUrl),
				TumLiveContext: &testutils.TUMLiveContextAdmin,
				DaoWrapper: dao.DaoWrapper{
					CoursesDao: testutils.GetCoursesMock(t),
					StatisticsDao: func() dao.StatisticsDao {
						statisticsMock := mock_dao.NewMockStatisticsDao(gomock.NewController(t))
						statisticsMock.
							EXPECT().
							GetStudentActivityCourseStats(testutils.CourseFPV.ID, false).
							Return(stats, nil).
							AnyTimes()
						return statisticsMock
					}(),
				},
				Before: func() {
					resp.ChartType = "line"
					resp.Data.Datasets[0].Label = "VoD"
					resp.Data.Datasets[0].Data = stats
					resp.Data.Datasets[0].BorderColor = "#2a7dd1"
					resp.Data.Datasets[0].BackgroundColor = ""

					res, _ = json.Marshal(resp)
				},
				ExpectedCode:     http.StatusOK,
				ExpectedResponse: res,
			},
			"success numStudents": {
				Method:         http.MethodGet,
				Url:            fmt.Sprintf("%s?interval=numStudents", baseUrl),
				TumLiveContext: &testutils.TUMLiveContextAdmin,
				DaoWrapper: dao.DaoWrapper{
					CoursesDao: testutils.GetCoursesMock(t),
					StatisticsDao: func() dao.StatisticsDao {
						statisticsMock := mock_dao.NewMockStatisticsDao(gomock.NewController(t))
						statisticsMock.
							EXPECT().
							GetCourseNumStudents(testutils.CourseFPV.ID).
							Return(numStudents, nil).
							AnyTimes()
						return statisticsMock
					}(),
				},
				Before: func() {
					res, _ = json.Marshal(gin.H{"res": numStudents})
				},
				ExpectedCode:     http.StatusOK,
				ExpectedResponse: res,
			},
			"success vodViews": {
				Method:         http.MethodGet,
				Url:            fmt.Sprintf("%s?interval=vodViews", baseUrl),
				TumLiveContext: &testutils.TUMLiveContextAdmin,
				DaoWrapper: dao.DaoWrapper{
					CoursesDao: testutils.GetCoursesMock(t),
					StatisticsDao: func() dao.StatisticsDao {
						statisticsMock := mock_dao.NewMockStatisticsDao(gomock.NewController(t))
						statisticsMock.
							EXPECT().
							GetCourseNumVodViews(testutils.CourseFPV.ID).
							Return(views, nil).
							AnyTimes()
						return statisticsMock
					}(),
				},
				Before: func() {
					res, _ = json.Marshal(gin.H{"res": views})
				},
				ExpectedCode:     http.StatusOK,
				ExpectedResponse: res,
			},
			"success liveViews": {
				Method:         http.MethodGet,
				Url:            fmt.Sprintf("%s?interval=liveViews", baseUrl),
				TumLiveContext: &testutils.TUMLiveContextAdmin,
				DaoWrapper: dao.DaoWrapper{
					CoursesDao: testutils.GetCoursesMock(t),
					StatisticsDao: func() dao.StatisticsDao {
						statisticsMock := mock_dao.NewMockStatisticsDao(gomock.NewController(t))
						statisticsMock.
							EXPECT().
							GetCourseNumLiveViews(testutils.CourseFPV.ID).
							Return(views, nil).
							AnyTimes()
						return statisticsMock
					}(),
				},
				Before: func() {
					res, _ = json.Marshal(gin.H{"res": views})
				},
				ExpectedCode:     http.StatusOK,
				ExpectedResponse: res,
			},
			"success allDays": {
				Method:         http.MethodGet,
				Url:            fmt.Sprintf("%s?interval=allDays", baseUrl),
				TumLiveContext: &testutils.TUMLiveContextAdmin,
				DaoWrapper: dao.DaoWrapper{
					CoursesDao: testutils.GetCoursesMock(t),
					StatisticsDao: func() dao.StatisticsDao {
						statisticsMock := mock_dao.NewMockStatisticsDao(gomock.NewController(t))
						statisticsMock.
							EXPECT().
							GetCourseNumVodViewsPerDay(testutils.CourseFPV.ID).
							Return(stats, nil).
							AnyTimes()
						return statisticsMock
					}(),
				},
				Before: func() {
					resp.ChartType = "bar"
					resp.Data.Datasets[0].Label = "views"
					resp.Data.Datasets[0].Data = stats
					resp.Data.Datasets[0].BorderColor = "#427dbd"
					resp.Data.Datasets[0].BackgroundColor = "#d12a5c"

					res, _ = json.Marshal(resp)
				},
				ExpectedCode:     http.StatusOK,
				ExpectedResponse: res,
			},
		}

		for _, interval := range intervals {
			testCases["can not get statistics - "+interval] = testutils.TestCase{
				Method: http.MethodGet,
				Url:    fmt.Sprintf("%s?interval=%s", baseUrl, interval),
				DaoWrapper: dao.DaoWrapper{
					CoursesDao: testutils.GetCoursesMock(t),
					StatisticsDao: func() dao.StatisticsDao {
						statisticsMock := mock_dao.NewMockStatisticsDao(gomock.NewController(t))
						statisticsMock.
							EXPECT().
							GetCourseStatsWeekdays(testutils.CourseFPV.ID).
							Return([]dao.Stat{}, errors.New("")).
							AnyTimes()

						statisticsMock.
							EXPECT().
							GetCourseStatsHourly(testutils.CourseFPV.ID).
							Return([]dao.Stat{}, errors.New("")).
							AnyTimes()

						statisticsMock.
							EXPECT().
							GetStudentActivityCourseStats(testutils.CourseFPV.ID, true).
							Return([]dao.Stat{}, errors.New("")).
							AnyTimes()

						statisticsMock.
							EXPECT().
							GetStudentActivityCourseStats(testutils.CourseFPV.ID, false).
							Return([]dao.Stat{}, errors.New("")).
							AnyTimes()

						statisticsMock.
							EXPECT().
							GetCourseNumStudents(testutils.CourseFPV.ID).
							Return(int64(0), errors.New("")).
							AnyTimes()

						statisticsMock.
							EXPECT().
							GetCourseNumVodViews(testutils.CourseFPV.ID).
							Return(0, errors.New("")).
							AnyTimes()

						statisticsMock.
							EXPECT().
							GetCourseNumLiveViews(testutils.CourseFPV.ID).
							Return(0, errors.New("")).
							AnyTimes()

						statisticsMock.
							EXPECT().
							GetCourseNumVodViewsPerDay(testutils.CourseFPV.ID).
							Return([]dao.Stat{}, errors.New("")).
							AnyTimes()
						return statisticsMock
					}(),
				},
				TumLiveContext: &testutils.TUMLiveContextAdmin,
				ExpectedCode:   http.StatusInternalServerError,
			}
		}

		testCases.Run(t, configGinCourseRouter)
	})
}
