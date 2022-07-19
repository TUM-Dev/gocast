package api

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/golang/mock/gomock"
	"github.com/joschahenningsen/TUM-Live/dao"
	"github.com/joschahenningsen/TUM-Live/mock_dao"
	"github.com/joschahenningsen/TUM-Live/model"
	"github.com/joschahenningsen/TUM-Live/tools/testutils"
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

		testutils.TestCases{
			"invalid year": {
				Method:       http.MethodGet,
				Url:          fmt.Sprintf("/api/download_ics/%s/%s/%s/events.ics", "abc", term, slug),
				ExpectedCode: http.StatusBadRequest,
			},
			"can not get course by year,term,slug": {
				Method: http.MethodGet,
				Url:    url,
				DaoWrapper: dao.DaoWrapper{
					CoursesDao: func() dao.CoursesDao {
						courseMock := mock_dao.NewMockCoursesDao(gomock.NewController(t))
						courseMock.
							EXPECT().
							GetCourseBySlugYearAndTerm(gomock.Any(), slug, term, year).
							Return(model.Course{}, errors.New("")).
							AnyTimes()
						return courseMock
					}(),
				},
				ExpectedCode: http.StatusBadRequest,
			},
			"success": {
				Method: http.MethodGet,
				Url:    url,
				DaoWrapper: dao.DaoWrapper{
					CoursesDao: func() dao.CoursesDao {
						courseMock := mock_dao.NewMockCoursesDao(gomock.NewController(t))
						courseMock.
							EXPECT().
							GetCourseBySlugYearAndTerm(gomock.Any(), slug, term, year).
							Return(testutils.CourseFPV, nil).
							AnyTimes()
						return courseMock
					}(),
				},
				ExpectedHeader: testutils.HttpHeader{
					"content-type":              "text/calendar",
					"Content-Transfer-Encoding": "binary",
					"Content-Disposition":       fmt.Sprintf("attachment; filename=%s%s%d.ics", slug, term, year),
				},
				ExpectedCode:     http.StatusOK,
				ExpectedResponse: res.Bytes(),
			},
		}.Run(t, configGinDownloadICSRouter)
	})
}
