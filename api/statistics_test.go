package api

import (
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/golang/mock/gomock"
	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/mock_dao"
	"github.com/TUM-Dev/gocast/model"
	"github.com/TUM-Dev/gocast/tools"
	"github.com/TUM-Dev/gocast/tools/testutils"
	"github.com/matthiasreumann/gomino"
	"gorm.io/gorm"
	"net/http"
	"testing"
)

func TestStatistics(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("GET/api/course/:courseID/stats", func(t *testing.T) {
		baseUrl := fmt.Sprintf("/api/course/%d/stats", testutils.CourseFPV.ID)

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

		testCases := gomino.TestCases{
			"invalid body": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						CoursesDao: testutils.GetCoursesMock(t),
					}
					configGinCourseRouter(r, wrapper)
				},
				Url:          baseUrl,
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				ExpectedCode: http.StatusBadRequest,
			},
			"courseID 0, not admin": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						CoursesDao: func() dao.CoursesDao {
							coursesMock := mock_dao.NewMockCoursesDao(gomock.NewController(t))
							coursesMock.
								EXPECT().
								GetCourseById(gomock.Any(), uint(0)).
								Return(testutils.CourseFPV, nil).
								AnyTimes()
							return coursesMock
						}(),
					}
					configGinCourseRouter(r, wrapper)
				},
				Url:          fmt.Sprintf("/api/course/%d/stats%s", 0, "?interval=week"),
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextStudent)),
				ExpectedCode: http.StatusForbidden,
			},
			"invalid interval": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						CoursesDao: testutils.GetCoursesMock(t),
					}
					configGinCourseRouter(r, wrapper)
				},
				Url:          fmt.Sprintf("%s?interval=%s", baseUrl, "century"),
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				ExpectedCode: http.StatusBadRequest,
			},
			"success week,day": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
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
					}
					configGinCourseRouter(r, wrapper)
				},
				Url:         fmt.Sprintf("%s?interval=week", baseUrl),
				Middlewares: testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				Before: func() {
					resp.Data = chartJsData{Datasets: []chartJsDataset{newChartJsDataset()}}
					resp.ChartType = "bar"
					resp.Data.Datasets[0].Label = "Sum(viewers)"
					resp.Data.Datasets[0].Data = stats
				},
				ExpectedCode:     http.StatusOK,
				ExpectedResponse: &resp,
			},
			"success courseID 0, week,day": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
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
					}
					configGinCourseRouter(r, wrapper)
				},
				Url:         fmt.Sprintf("/api/course/%d/stats%s", 0, "?interval=week"),
				Middlewares: testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				Before: func() {
					resp.Data = chartJsData{Datasets: []chartJsDataset{newChartJsDataset()}}
					resp.ChartType = "bar"
					resp.Data.Datasets[0].Label = "Sum(viewers)"
					resp.Data.Datasets[0].Data = stats
				},
				ExpectedCode:     http.StatusOK,
				ExpectedResponse: &resp,
			},
			"success hour": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
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
					}
					configGinCourseRouter(r, wrapper)
				},
				Url:         fmt.Sprintf("%s?interval=hour", baseUrl),
				Middlewares: testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				Before: func() {
					resp.Data = chartJsData{Datasets: []chartJsDataset{newChartJsDataset()}}
					resp.ChartType = "bar"
					resp.Data.Datasets[0].Label = "Sum(viewers)"
					resp.Data.Datasets[0].Data = stats
				},
				ExpectedCode:     http.StatusOK,
				ExpectedResponse: &resp,
			},
			"success activity-live": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
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
					}
					configGinCourseRouter(r, wrapper)
				},
				Url:         fmt.Sprintf("%s?interval=activity-live", baseUrl),
				Middlewares: testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				Before: func() {
					resp.Data = chartJsData{Datasets: []chartJsDataset{newChartJsDataset()}}
					resp.ChartType = "line"
					resp.Data.Datasets[0].Label = "Live"
					resp.Data.Datasets[0].Data = stats
					resp.Data.Datasets[0].BorderColor = "#d12a5c"
					resp.Data.Datasets[0].BackgroundColor = ""
				},
				ExpectedCode:     http.StatusOK,
				ExpectedResponse: &resp,
			},
			"success activity-vod": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
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
					}
					configGinCourseRouter(r, wrapper)
				},
				Url:         fmt.Sprintf("%s?interval=activity-vod", baseUrl),
				Middlewares: testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				Before: func() {
					resp.Data = chartJsData{Datasets: []chartJsDataset{newChartJsDataset()}}
					resp.ChartType = "line"
					resp.Data.Datasets[0].Label = "VoD"
					resp.Data.Datasets[0].Data = stats
					resp.Data.Datasets[0].BorderColor = "#2a7dd1"
					resp.Data.Datasets[0].BackgroundColor = ""
				},
				ExpectedCode:     http.StatusOK,
				ExpectedResponse: &resp,
			},
			"success numStudents": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
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
					}
					configGinCourseRouter(r, wrapper)
				},
				Url:              fmt.Sprintf("%s?interval=numStudents", baseUrl),
				Middlewares:      testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				ExpectedCode:     http.StatusOK,
				ExpectedResponse: gin.H{"res": numStudents},
			},
			"success vodViews": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
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
					}
					configGinCourseRouter(r, wrapper)
				},
				Url:              fmt.Sprintf("%s?interval=vodViews", baseUrl),
				Middlewares:      testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				ExpectedCode:     http.StatusOK,
				ExpectedResponse: gin.H{"res": views},
			},
			"success liveViews": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
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
					}
					configGinCourseRouter(r, wrapper)
				},
				Url:              fmt.Sprintf("%s?interval=liveViews", baseUrl),
				Middlewares:      testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				ExpectedCode:     http.StatusOK,
				ExpectedResponse: gin.H{"res": views},
			},
			"success allDays": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
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
					}
					configGinCourseRouter(r, wrapper)
				},
				Url:         fmt.Sprintf("%s?interval=allDays", baseUrl),
				Middlewares: testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				Before: func() {
					resp.Data = chartJsData{Datasets: []chartJsDataset{newChartJsDataset()}}
					resp.ChartType = "bar"
					resp.Data.Datasets[0].Label = "views"
					resp.Data.Datasets[0].Data = stats
					resp.Data.Datasets[0].BorderColor = "#427dbd"
					resp.Data.Datasets[0].BackgroundColor = "#d12a5c"
				},
				ExpectedCode:     http.StatusOK,
				ExpectedResponse: &resp,
			},
		}

		for _, interval := range intervals {
			testCases["can not get statistics - "+interval] = &gomino.TestCase{
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
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
					}
					configGinCourseRouter(r, wrapper)
				},
				Url:          fmt.Sprintf("%s?interval=%s", baseUrl, interval),
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				ExpectedCode: http.StatusInternalServerError,
			}
		}

		testCases.Method(http.MethodGet).Run(t, testutils.Equal)
	})
}
