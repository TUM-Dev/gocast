package apiv2

import (
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/mock/gomock"
	"gorm.io/gorm"

	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/mock_dao"
	"github.com/TUM-Dev/gocast/model"
	"github.com/TUM-Dev/gocast/tools"
)

func TestCourseCalendarRouteOf(t *testing.T) {
	for _, tc := range []struct {
		method, path string
		want         *courseCalendarRoute
	}{
		{"GET", "/api/v2/courses/2022/W/fpv/events.ics", &courseCalendarRoute{year: "2022", term: "W", slug: "fpv"}},
		{"HEAD", "/api/v2/courses/2022/W/fpv/events.ics", &courseCalendarRoute{year: "2022", term: "W", slug: "fpv"}},
		{"POST", "/api/v2/courses/2022/W/fpv/events.ics", nil},
		{"GET", "/api/v2/courses/2022/W/fpv", nil},
		{"GET", "/api/v2/courses/2022/W/fpv/other.ics", nil},
		{"GET", "/api/v2/courses/2022//fpv/events.ics", nil},
		{"GET", "/api/v2/courses/1/streams/7/attachments", nil},
	} {
		got, ok := courseCalendarRouteOf(httptest.NewRequest(tc.method, tc.path, nil))
		if (tc.want == nil) == ok || (tc.want != nil && got != *tc.want) {
			t.Errorf("%s %s: got %+v, %v; want %+v", tc.method, tc.path, got, ok, tc.want)
		}
	}
}

// The feed at both addresses, through the same registrations the server makes: the
// v2 one by the dispatch Proxy performs, the original one by RegisterLegacyCalendar.
func calendarRouter(api *API) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Any("/api/v2/*any", func(c *gin.Context) {
		route, ok := courseCalendarRouteOf(c.Request)
		if !ok {
			c.Status(http.StatusTeapot)
			return
		}
		api.handleCourseCalendar(c, route.year, route.term, route.slug)
	})
	api.RegisterLegacyCalendar(r)
	return r
}

func TestCourseCalendar(t *testing.T) {
	restore := func() func() {
		prev := tools.Cfg.WebUrl
		tools.Cfg.WebUrl = "https://tum.live"
		return func() { tools.Cfg.WebUrl = prev }
	}()
	t.Cleanup(restore)

	start := time.Date(2022, 10, 17, 10, 15, 0, 0, time.UTC)
	course := model.Course{
		Model: gorm.Model{ID: 40}, Name: "Funktionale Programmierung", Slug: "fpv", Year: 2022, TeachingTerm: "W",
		// Private to enrolled users, yet the feed is served to anyone: see the handler.
		Visibility: "enrolled",
		Streams: []model.Stream{{
			Model: gorm.Model{ID: 7, CreatedAt: start.Add(-48 * time.Hour)},
			Name:  "VL 1", Start: start, End: start.Add(90 * time.Minute), RoomName: "MI HS1", RoomCode: "5602.EG.001",
		}},
	}
	newAPI := func(t *testing.T, mock func(*mock_dao.MockCoursesDao)) *API {
		courses := mock_dao.NewMockCoursesDao(gomock.NewController(t))
		mock(courses)
		return &API{dao: dao.DaoWrapper{CoursesDao: courses}, log: slog.Default()}
	}
	get := func(api *API, path string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		calendarRouter(api).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		return rec
	}

	for _, path := range []string{
		"/api/v2/courses/2022/W/fpv/events.ics",
		"/api/download_ics/2022/W/fpv/events.ics",
	} {
		t.Run("serves the feed at "+path+" with no credentials", func(t *testing.T) {
			api := newAPI(t, func(c *mock_dao.MockCoursesDao) {
				c.EXPECT().GetCourseBySlugYearAndTerm(gomock.Any(), "fpv", "W", 2022).Return(course, nil)
			})

			rec := get(api, path)

			if rec.Code != http.StatusOK {
				t.Fatalf("got %d: %s", rec.Code, rec.Body)
			}
			if got := rec.Header().Get("Content-Type"); got != "text/calendar" {
				t.Errorf("Content-Type %q", got)
			}
			if got := rec.Header().Get("Content-Disposition"); got != "attachment; filename=fpvW2022.ics" {
				t.Errorf("Content-Disposition %q", got)
			}
			if got := rec.Header().Get("Content-Transfer-Encoding"); got != "binary" {
				t.Errorf("Content-Transfer-Encoding %q", got)
			}
			// What v1 produced for this course, line by line, so a template or field
			// change cannot pass for the same feed.
			body := strings.ReplaceAll(rec.Body.String(), "\r\n", "\n")
			for _, want := range []string{
				"BEGIN:VCALENDAR\nMETHOD:PUBLISH\nVERSION:2.0\n",
				"BEGIN:VEVENT\nUID:7@live.rbg.tum.de\n",
				"DTSTART;TZID=W. Europe Standard Time:20221017T101500\n",
				"DTEND;TZID=W. Europe Standard Time:20221017T114500\n",
				"DTSTAMP:20221015T101500\n",
				"URL:https://tum.live/2022/W/fpv\n",
				"LOCATION:MI HS1 5602.EG.001\n",
				"SUMMARY:Funktionale Programmierung\n",
				"DESCRIPTION:VL 1\\nhttps://tum.live/2022/W/fpv\\nMI HS1 5602.EG.001\nEND:VEVENT\nEND:VCALENDAR",
			} {
				if !strings.Contains(body, want) {
					t.Errorf("the feed lacks %q:\n%s", want, body)
				}
			}
		})
	}

	t.Run("both addresses answer identically", func(t *testing.T) {
		api := newAPI(t, func(c *mock_dao.MockCoursesDao) {
			c.EXPECT().GetCourseBySlugYearAndTerm(gomock.Any(), "fpv", "W", 2022).Return(course, nil).Times(2)
		})
		a, b := get(api, "/api/v2/courses/2022/W/fpv/events.ics"), get(api, "/api/download_ics/2022/W/fpv/events.ics")
		if a.Body.String() != b.Body.String() || a.Code != http.StatusOK || b.Code != http.StatusOK {
			t.Errorf("v2 %d and original %d differ", a.Code, b.Code)
		}
	})

	t.Run("a course with no lectures is an empty calendar", func(t *testing.T) {
		api := newAPI(t, func(c *mock_dao.MockCoursesDao) {
			c.EXPECT().GetCourseBySlugYearAndTerm(gomock.Any(), "fpv", "W", 2022).Return(model.Course{Slug: "fpv", Year: 2022, TeachingTerm: "W"}, nil)
		})
		rec := get(api, "/api/download_ics/2022/W/fpv/events.ics")
		if rec.Code != http.StatusOK || strings.Contains(rec.Body.String(), "VEVENT") || !strings.Contains(rec.Body.String(), "END:VCALENDAR") {
			t.Errorf("got %d: %s", rec.Code, rec.Body)
		}
	})

	t.Run("an invalid year is a 400", func(t *testing.T) {
		api := newAPI(t, func(*mock_dao.MockCoursesDao) {})
		for _, path := range []string{"/api/v2/courses/abc/W/fpv/events.ics", "/api/download_ics/abc/W/fpv/events.ics"} {
			if rec := get(api, path); rec.Code != http.StatusBadRequest {
				t.Errorf("%s: got %d", path, rec.Code)
			}
		}
	})

	t.Run("an unknown course is a 404 and a broken lookup a 500", func(t *testing.T) {
		api := newAPI(t, func(c *mock_dao.MockCoursesDao) {
			c.EXPECT().GetCourseBySlugYearAndTerm(gomock.Any(), "nope", "W", 2022).Return(model.Course{}, gorm.ErrRecordNotFound)
			c.EXPECT().GetCourseBySlugYearAndTerm(gomock.Any(), "boom", "W", 2022).Return(model.Course{}, errors.New("db down"))
		})
		if rec := get(api, "/api/download_ics/2022/W/nope/events.ics"); rec.Code != http.StatusNotFound {
			t.Errorf("got %d", rec.Code)
		}
		if rec := get(api, "/api/v2/courses/2022/W/boom/events.ics"); rec.Code != http.StatusInternalServerError {
			t.Errorf("got %d", rec.Code)
		}
	})
}
