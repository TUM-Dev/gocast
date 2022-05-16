package api

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/golang/mock/gomock"
	"github.com/joschahenningsen/TUM-Live/dao"
	"github.com/joschahenningsen/TUM-Live/mock_dao"
	"github.com/joschahenningsen/TUM-Live/model"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
	"html/template"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestDownloadICS(t *testing.T) {
	w := httptest.NewRecorder()
	c, r := gin.CreateTestContext(w)

	courseMock := mock_dao.NewMockCoursesDao(gomock.NewController(t))

	configGinDownloadICSRouter(r, dao.DaoWrapper{CoursesDao: courseMock})

	templates, _ := template.ParseFS(staticFS, "template/*.gotemplate")

	t.Run("year not a int", func(t *testing.T) {
		slug, term, year := "fda", "S", "abc"
		c.Request, _ = http.NewRequest(http.MethodGet,
			fmt.Sprintf("/api/download_ics/%s/%s/%s/events.ics", year, term, slug), nil)
		r.ServeHTTP(w, c.Request)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("coursesDao returns error", func(t *testing.T) {
		year, term, slug := 2022, "S", "fda"
		courseMock.
			EXPECT().
			GetCourseBySlugYearAndTerm(gomock.Any(), slug, term, year).
			Return(model.Course{}, errors.New("")).
			AnyTimes()

		c.Request, _ = http.NewRequest(http.MethodGet,
			fmt.Sprintf("/api/download_ics/%d/%s/%s/events.ics", year, term, slug), nil)
		r.ServeHTTP(w, c.Request)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("success", func(t *testing.T) {
		var icsContentExpected bytes.Buffer
		year, term, slug := 2022, "S", "fda"
		createdAt := time.Now()
		start, end := createdAt, createdAt.Add(time.Hour)

		course := model.Course{
			Name:         "Foundations in Data Analysis [MA4800]",
			Year:         year,
			TeachingTerm: term,
			Slug:         slug,
			Streams: []model.Stream{
				{
					Model:    gorm.Model{ID: 1, CreatedAt: createdAt},
					Name:     "Lecture 1",
					Start:    start,
					End:      end,
					RoomName: "01.11.018",
					RoomCode: "01.11.018"},
			},
		}

		courseMock.
			EXPECT().
			GetCourseBySlugYearAndTerm(gomock.Any(), slug, term, year).
			Return(course, nil).
			AnyTimes()

		var calendarEntries []CalendarEntry
		for _, s := range course.Streams {
			calendarEntries = append(calendarEntries, streamToCalendarEntry(s, course))
		}

		_ = templates.ExecuteTemplate(&icsContentExpected, "ics.gotemplate", calendarEntries)

		c.Request, _ = http.NewRequest(http.MethodGet,
			fmt.Sprintf("/api/download_ics/%d/%s/%s/events.ics", year, term, slug), nil)
		r.ServeHTTP(w, c.Request)

		assert.Equal(t, "text/calendar", w.Header().Get("content-type"))
		assert.Equal(t, "binary", w.Header().Get("Content-Transfer-Encoding"))
		assert.Equal(t, fmt.Sprintf("attachment; filename=%s%s%d.ics", slug, term, year), w.Header().Get("Content-Disposition"))
		assert.Equal(t, icsContentExpected.String(), w.Body.String())
		assert.Equal(t, http.StatusOK, w.Code)
	})
}
