package api

import (
	"bytes"
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
	"html/template"
	"net/http"
	"testing"
)

func TestDownloadICS(t *testing.T) {
	templates, _ := template.ParseFS(staticFS, "template/*.gotemplate")

	t.Run("GET/api/download_ics/:year/:term/:slug/events.ics", func(t *testing.T) {
		year, term, slug := 2022, "W", "fpv"
		url := fmt.Sprintf("/api/download_ics/%d/%s/%s/events.ics", year, term, slug)

		var res bytes.Buffer
		var calendarEntries []CalendarEntry
		for _, s := range testutils.CourseFPV.Streams {
			calendarEntries = append(calendarEntries, streamToCalendarEntry(s, testutils.CourseFPV))
		}

		_ = templates.ExecuteTemplate(&res, "ics.gotemplate", calendarEntries)

		gomino.TestCases{
			"invalid year": {
				Router: func(r *gin.Engine) {
					configGinDownloadICSRouter(r, dao.DaoWrapper{})
				},
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler),
				Url:          fmt.Sprintf("/api/download_ics/%s/%s/%s/events.ics", "abc", term, slug),
				ExpectedCode: http.StatusBadRequest,
			},
			"can not get course by year,term,slug": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						CoursesDao: func() dao.CoursesDao {
							courseMock := mock_dao.NewMockCoursesDao(gomock.NewController(t))
							courseMock.
								EXPECT().
								GetCourseBySlugYearAndTerm(gomock.Any(), slug, term, year).
								Return(model.Course{}, errors.New("")).
								AnyTimes()
							return courseMock
						}(),
					}
					configGinDownloadICSRouter(r, wrapper)
				},
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler),
				ExpectedCode: http.StatusBadRequest,
			},
			"success": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						CoursesDao: func() dao.CoursesDao {
							courseMock := mock_dao.NewMockCoursesDao(gomock.NewController(t))
							courseMock.
								EXPECT().
								GetCourseBySlugYearAndTerm(gomock.Any(), slug, term, year).
								Return(testutils.CourseFPV, nil).
								AnyTimes()
							return courseMock
						}(),
					}
					configGinDownloadICSRouter(r, wrapper)
				},
				Middlewares: testutils.GetMiddlewares(tools.ErrorHandler),
				ExpectedHeader: gomino.HttpHeader{
					"content-type":              "text/calendar",
					"Content-Transfer-Encoding": "binary",
					"Content-Disposition":       fmt.Sprintf("attachment; filename=%s%s%d.ics", slug, term, year),
				},
				ExpectedCode:     http.StatusOK,
				ExpectedResponse: res.Bytes(),
			}}.
			Method(http.MethodGet).
			Url(url).
			Run(t, testutils.Equal)
	})
}
