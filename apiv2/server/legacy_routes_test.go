package apiv2

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"text/template"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	"gorm.io/gorm"

	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/mock_dao"
	"github.com/TUM-Dev/gocast/model"
	"github.com/TUM-Dev/gocast/tools"
	"github.com/TUM-Dev/gocast/tools/testutils"
)

// legacyRouter wires the routes the way main does: the v2 addresses through Handler,
// the original ones through RegisterLegacyRoutes. The user is what InitContext would
// have found from the session cookie.
func legacyRouter(a *API, ctx tools.TUMLiveContext) *tools.Router {
	r := tools.NewRouter()
	r.Use(testutils.InjectContext(ctx))
	r.Handle("/api/v2/", a.Handler())
	a.RegisterLegacyRoutes(r)
	return r
}

func serve(r http.Handler, method, path, body string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(method, path, strings.NewReader(body)))
	return rec
}

type fakeSelfStreams struct {
	err     error
	started []model.Stream
}

func (f *fakeSelfStreams) RequestSelfStream(_ context.Context, s model.Stream) error {
	f.started = append(f.started, s)
	return f.err
}

func TestSelfStreamPublish(t *testing.T) {
	paths := []string{"/api/v2/selfstream/on-publish", "/api/selfstream/onPublish"}
	const good = `{"action":"publish","query":"secret=KEY/fpv-7"}`

	setup := func(t *testing.T, streams *mock_dao.MockStreamsDao, courses *mock_dao.MockCoursesDao, ss *fakeSelfStreams) *tools.Router {
		a := &API{dao: dao.DaoWrapper{StreamsDao: streams, CoursesDao: courses}, log: slog.Default()}
		if ss != nil {
			a.selfStreams = ss
		}
		return legacyRouter(a, tools.TUMLiveContext{})
	}
	mocks := func(t *testing.T) (*mock_dao.MockStreamsDao, *mock_dao.MockCoursesDao) {
		ctrl := gomock.NewController(t)
		return mock_dao.NewMockStreamsDao(ctrl), mock_dao.NewMockCoursesDao(ctrl)
	}
	stream := model.Stream{Model: gorm.Model{ID: 7}, CourseID: 40}

	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			t.Run("starts the stream when key and slug match", func(t *testing.T) {
				streams, courses := mocks(t)
				streams.EXPECT().GetStreamByKey(gomock.Any(), "KEY").Return(stream, nil)
				courses.EXPECT().GetCourseById(gomock.Any(), uint(40)).Return(model.Course{Slug: "fpv"}, nil)
				ss := &fakeSelfStreams{}
				rec := serve(setup(t, streams, courses, ss), http.MethodPost, path, good)
				assert.Equal(t, http.StatusOK, rec.Code)
				assert.Len(t, ss.started, 1)
			})
			t.Run("approves other actions without checking", func(t *testing.T) {
				streams, courses := mocks(t)
				rec := serve(setup(t, streams, courses, &fakeSelfStreams{}), http.MethodPost, path, `{"action":"read"}`)
				assert.Equal(t, http.StatusOK, rec.Code)
			})
			t.Run("rejects a body that is not JSON", func(t *testing.T) {
				streams, courses := mocks(t)
				rec := serve(setup(t, streams, courses, nil), http.MethodPost, path, `nope`)
				assert.Equal(t, http.StatusBadRequest, rec.Code)
			})
			t.Run("rejects a query in the wrong format", func(t *testing.T) {
				streams, courses := mocks(t)
				rec := serve(setup(t, streams, courses, nil), http.MethodPost, path, `{"action":"publish","query":"secret=KEY"}`)
				assert.Equal(t, http.StatusBadRequest, rec.Code)
			})
			t.Run("forbids a slug of another stream", func(t *testing.T) {
				streams, courses := mocks(t)
				streams.EXPECT().GetStreamByKey(gomock.Any(), "KEY").Return(stream, nil)
				courses.EXPECT().GetCourseById(gomock.Any(), uint(40)).Return(model.Course{Slug: "gbs"}, nil)
				ss := &fakeSelfStreams{}
				rec := serve(setup(t, streams, courses, ss), http.MethodPost, path, good)
				assert.Equal(t, http.StatusForbidden, rec.Code)
				assert.Empty(t, ss.started)
			})
			t.Run("500 for an unknown key", func(t *testing.T) {
				streams, courses := mocks(t)
				streams.EXPECT().GetStreamByKey(gomock.Any(), "KEY").Return(model.Stream{}, errors.New("nope"))
				rec := serve(setup(t, streams, courses, nil), http.MethodPost, path, good)
				assert.Equal(t, http.StatusInternalServerError, rec.Code)
			})
			t.Run("500 when the runners refuse", func(t *testing.T) {
				streams, courses := mocks(t)
				streams.EXPECT().GetStreamByKey(gomock.Any(), "KEY").Return(stream, nil)
				courses.EXPECT().GetCourseById(gomock.Any(), uint(40)).Return(model.Course{Slug: "fpv"}, nil)
				rec := serve(setup(t, streams, courses, &fakeSelfStreams{err: errors.New("busy")}), http.MethodPost, path, good)
				assert.Equal(t, http.StatusInternalServerError, rec.Code)
			})
			t.Run("only POST", func(t *testing.T) {
				streams, courses := mocks(t)
				rec := serve(setup(t, streams, courses, nil), http.MethodGet, path, "")
				assert.Contains(t, []int{http.StatusNotFound, http.StatusMethodNotAllowed}, rec.Code)
			})
		})
	}
}

func TestTokenProxy(t *testing.T) {
	prev := tools.Cfg.IngestBase
	tools.Cfg.IngestBase = "rtmp://ingest.example.com/"
	t.Cleanup(func() { tools.Cfg.IngestBase = prev })

	lecturerToken := model.Token{Scope: model.TokenScopeLecturer, UserID: 31}
	lecturer := model.User{Model: gorm.Model{ID: 31}, Role: model.LecturerType}

	type mocks struct {
		tokens  *mock_dao.MockTokenDao
		users   *mock_dao.MockUsersDao
		streams *mock_dao.MockStreamsDao
	}
	run := func(t *testing.T, path string, setup func(m mocks)) *httptest.ResponseRecorder {
		ctrl := gomock.NewController(t)
		m := mocks{mock_dao.NewMockTokenDao(ctrl), mock_dao.NewMockUsersDao(ctrl), mock_dao.NewMockStreamsDao(ctrl)}
		setup(m)
		a := &API{dao: dao.DaoWrapper{TokenDao: m.tokens, UsersDao: m.users, StreamsDao: m.streams}, log: slog.Default()}
		return serve(legacyRouter(a, tools.TUMLiveContext{}), http.MethodPost, path, "")
	}

	for _, prefix := range []string{"/api/v2/token/proxy/", "/api/token/proxy/"} {
		t.Run(prefix, func(t *testing.T) {
			t.Run("returns the ingest URL", func(t *testing.T) {
				rec := run(t, prefix+"TOK?slug=fpv", func(m mocks) {
					m.tokens.EXPECT().GetToken("TOK").Return(lecturerToken, nil)
					m.users.EXPECT().GetUserByID(gomock.Any(), uint(31)).Return(lecturer, nil)
					m.streams.EXPECT().GetSoonStartingStreamInfo(gomock.Any(), "fpv", gomock.Any(), gomock.Any()).Return("KEY", "fpv-7", nil)
				})
				assert.Equal(t, http.StatusOK, rec.Code)
				assert.JSONEq(t, `{"url":"rtmp://ingest.example.com/fpv-7?secret=KEY/fpv-7"}`, rec.Body.String())
			})
			t.Run("unknown token", func(t *testing.T) {
				rec := run(t, prefix+"TOK", func(m mocks) {
					m.tokens.EXPECT().GetToken("TOK").Return(model.Token{}, errors.New("nope"))
				})
				assert.Equal(t, http.StatusBadRequest, rec.Code)
				assert.JSONEq(t, `{"status":400,"message":"invalid token"}`, rec.Body.String())
			})
			t.Run("token of another scope", func(t *testing.T) {
				rec := run(t, prefix+"TOK", func(m mocks) {
					m.tokens.EXPECT().GetToken("TOK").Return(model.Token{Scope: "admin"}, nil)
				})
				assert.Equal(t, http.StatusUnauthorized, rec.Code)
				assert.JSONEq(t, `{"status":401,"message":"invalid scope"}`, rec.Body.String())
			})
			t.Run("user lookup fails", func(t *testing.T) {
				rec := run(t, prefix+"TOK", func(m mocks) {
					m.tokens.EXPECT().GetToken("TOK").Return(lecturerToken, nil)
					m.users.EXPECT().GetUserByID(gomock.Any(), uint(31)).Return(model.User{}, errors.New("db"))
				})
				assert.Equal(t, http.StatusInternalServerError, rec.Code)
				assert.JSONEq(t, `{"status":500,"message":"could not get user","error":"db"}`, rec.Body.String())
			})
			t.Run("user is no lecturer", func(t *testing.T) {
				rec := run(t, prefix+"TOK", func(m mocks) {
					m.tokens.EXPECT().GetToken("TOK").Return(lecturerToken, nil)
					m.users.EXPECT().GetUserByID(gomock.Any(), uint(31)).Return(model.User{Role: model.StudentType}, nil)
				})
				assert.Equal(t, http.StatusUnauthorized, rec.Code)
				assert.JSONEq(t, `{"status":401,"message":"user is not a lecturer or admin"}`, rec.Body.String())
			})
			t.Run("no stream soon", func(t *testing.T) {
				rec := run(t, prefix+"TOK", func(m mocks) {
					m.tokens.EXPECT().GetToken("TOK").Return(lecturerToken, nil)
					m.users.EXPECT().GetUserByID(gomock.Any(), uint(31)).Return(lecturer, nil)
					m.streams.EXPECT().GetSoonStartingStreamInfo(gomock.Any(), "", gomock.Any(), gomock.Any()).Return("", "", nil)
				})
				assert.Equal(t, http.StatusNotFound, rec.Code)
				assert.JSONEq(t, `{"status":404,"message":"no stream found"}`, rec.Body.String())
			})
		})
	}

	t.Run("a v2 path with a nested or empty token is not the endpoint", func(t *testing.T) {
		// It goes to the gateway, which knows no such path.
		for _, p := range []string{"/api/v2/token/proxy/", "/api/v2/token/proxy/a/b"} {
			rec := run(t, p, func(m mocks) {})
			assert.Equal(t, http.StatusNotFound, rec.Code, p)
		}
	})
}

func TestLectureHallSchedule(t *testing.T) {
	results := []dao.CalendarResult{
		{StreamID: 1, Created: time.Now(), Start: time.Now(), End: time.Now(), CourseName: "FPV", LectureHallName: "HS1"},
		{StreamID: 2, Created: time.Now(), Start: time.Now(), End: time.Now(), CourseName: "A & B"},
	}
	var want bytes.Buffer
	templ, err := template.ParseFS(courseImportTemplateFS, "template/ical.gotemplate")
	assert.NoError(t, err)
	assert.NoError(t, templ.ExecuteTemplate(&want, "ical.gotemplate", results))

	setup := func(t *testing.T, mock func(*mock_dao.MockLectureHallsDao), ctx *tools.TUMLiveContext) *tools.Router {
		halls := mock_dao.NewMockLectureHallsDao(gomock.NewController(t))
		mock(halls)
		a := &API{dao: dao.DaoWrapper{LectureHallsDao: halls}, log: slog.Default()}
		if ctx == nil {
			// No middleware at all: the context a deployment always has is missing.
			r := tools.NewRouter()
			a.RegisterLegacyRoutes(r)
			return r
		}
		return legacyRouter(a, *ctx)
	}

	for _, base := range []string{"/api/v2/lecture-halls/schedule.ics", "/api/schedule.ics"} {
		t.Run(base, func(t *testing.T) {
			t.Run("admin sees every hall asked for, byte for byte the template", func(t *testing.T) {
				ctx := testutils.TUMLiveContextAdmin
				r := setup(t, func(m *mock_dao.MockLectureHallsDao) {
					m.EXPECT().GetStreamsForLectureHallIcal(uint(0), []uint{1, 2}, false).Return(results, nil)
				}, &ctx)
				rec := serve(r, http.MethodGet, base+"?lecturehalls=1,2", "")
				assert.Equal(t, http.StatusOK, rec.Code)
				assert.Equal(t, "text/calendar", rec.Header().Get("Content-Type"))
				assert.Equal(t, want.String(), rec.Body.String())
				assert.Contains(t, rec.Body.String(), "SUMMARY:A & B")
			})
			t.Run("a student is limited to their own courses", func(t *testing.T) {
				ctx := testutils.TUMLiveContextStudent
				r := setup(t, func(m *mock_dao.MockLectureHallsDao) {
					m.EXPECT().GetStreamsForLectureHallIcal(testutils.Student.ID, []uint{1, 2}, false).Return(results, nil)
				}, &ctx)
				assert.Equal(t, http.StatusOK, serve(r, http.MethodGet, base+"?lecturehalls=1,2", "").Code)
			})
			t.Run("anonymous without the parameter gets all halls", func(t *testing.T) {
				ctx := testutils.TUMLiveContextUserNil
				r := setup(t, func(m *mock_dao.MockLectureHallsDao) {
					m.EXPECT().GetStreamsForLectureHallIcal(uint(0), []uint{}, true).Return(results, nil)
				}, &ctx)
				assert.Equal(t, http.StatusOK, serve(r, http.MethodGet, base, "").Code)
			})
			t.Run("hall ids must be numbers", func(t *testing.T) {
				ctx := testutils.TUMLiveContextUserNil
				r := setup(t, func(*mock_dao.MockLectureHallsDao) {}, &ctx)
				rec := serve(r, http.MethodGet, base+"?lecturehalls=1,x", "")
				assert.Equal(t, http.StatusBadRequest, rec.Code)
				assert.JSONEq(t, `{"status":400,"message":"Lecture Hall ID must be a number.","error":"strconv.Atoi: parsing \"x\": invalid syntax"}`, rec.Body.String())
			})
			t.Run("500 when the streams can not be read", func(t *testing.T) {
				ctx := testutils.TUMLiveContextUserNil
				r := setup(t, func(m *mock_dao.MockLectureHallsDao) {
					m.EXPECT().GetStreamsForLectureHallIcal(uint(0), []uint{1, 2}, false).Return(nil, errors.New("db"))
				}, &ctx)
				assert.Equal(t, http.StatusInternalServerError, serve(r, http.MethodGet, base+"?lecturehalls=1,2", "").Code)
			})
		})
	}

	t.Run("500 without a request context", func(t *testing.T) {
		r := setup(t, func(*mock_dao.MockLectureHallsDao) {}, nil)
		rec := serve(r, http.MethodGet, "/api/schedule.ics", "")
		assert.Equal(t, http.StatusInternalServerError, rec.Code)
		assert.JSONEq(t, `{"status":500,"message":"context should exist but doesn't"}`, rec.Body.String())
	})
}
