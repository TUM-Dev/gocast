package apiv2

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"go.uber.org/mock/gomock"
	"gorm.io/gorm"

	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/mock_dao"
	"github.com/TUM-Dev/gocast/model"
)

type mediaMocks struct {
	streams *mock_dao.MockStreamsDao
	courses *mock_dao.MockCoursesDao
}

// fakeUploads hands out one target, or an error, and records what it was asked for.
type fakeUploads struct {
	target UploadTarget
	err    error

	called   bool
	streamID uint
	version  model.StreamVersion
}

func (f *fakeUploads) Reserve(_ context.Context, streamID uint, version model.StreamVersion) (UploadTarget, error) {
	f.called, f.streamID, f.version = true, streamID, version
	return f.target, f.err
}

func newMediaAPI(t *testing.T, uploads Uploads) (*API, mediaMocks) {
	ctrl := gomock.NewController(t)
	m := mediaMocks{
		streams: mock_dao.NewMockStreamsDao(ctrl),
		courses: mock_dao.NewMockCoursesDao(ctrl),
	}
	return &API{
		dao:     dao.DaoWrapper{StreamsDao: m.streams, CoursesDao: m.courses},
		log:     slog.Default(),
		uploads: uploads,
	}, m
}

// fakeRunner serves a runner's upload endpoint with handler and returns the target
// pointing at it.
func fakeRunner(t *testing.T, handler http.HandlerFunc) UploadTarget {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	host, portStr, err := net.SplitHostPort(srv.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatal(err)
	}
	return UploadTarget{Host: host, Port: port, Token: "tok"}
}

func gatewayError(t *testing.T, body []byte) (code int, message string) {
	t.Helper()
	var got struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("body %s: %v", body, err)
	}
	return got.Code, got.Message
}

func TestLectureMediaRouteOf(t *testing.T) {
	got, ok := lectureUploadRouteOf(httptest.NewRequest(http.MethodPost, "/api/v2/courses/1/streams/7/media?type=COMB", nil))
	if !ok || got != (lectureUploadRoute{courseID: 1, streamID: 7, kind: uploadMedia}) {
		t.Errorf("got %+v, %v", got, ok)
	}
	for _, path := range []string{"/api/v2/courses/1/streams/7/media/2", "/api/v2/courses/1/streams/x/media"} {
		if _, ok := lectureUploadRouteOf(httptest.NewRequest(http.MethodPost, path, nil)); ok {
			t.Errorf("%s matched", path)
		}
	}
	if _, ok := lectureUploadRouteOf(httptest.NewRequest(http.MethodGet, "/api/v2/courses/1/streams/7/media", nil)); ok {
		t.Error("GET matched")
	}
}

func TestUploadLectureMedia(t *testing.T) {
	const path = "/api/v2/courses/1/streams/7/media?type=PRES"

	expectLecture := func(m mediaMocks) {
		m.courses.EXPECT().GetCourseById(gomock.Any(), uint(1)).Return(courseOne, nil)
		m.streams.EXPECT().GetStreamByID(gomock.Any(), "7").Return(courseOneLecture(), nil)
	}

	t.Run("streams the file to the reserved runner under its token", func(t *testing.T) {
		var gotPath, gotFile, gotCookie string
		uploads := &fakeUploads{}
		uploads.target = fakeRunner(t, func(w http.ResponseWriter, r *http.Request) {
			gotPath, gotCookie = r.URL.Path, r.Header.Get("Cookie")
			f, _, err := r.FormFile("file")
			if err != nil {
				t.Errorf("runner form: %v", err)
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			b, _ := io.ReadAll(f)
			gotFile = string(b)
		})
		api, m := newMediaAPI(t, uploads)
		expectLecture(m)

		req := uploadRequest(t, path, lectureAdmin, "lecture.mp4", "video/mp4")
		req.Header.Set("Cookie", "jwt=secret")
		rec := serveUpload(api, req)
		if rec.Code != http.StatusOK || rec.Body.String() != "{}" {
			t.Fatalf("status = %d: %s", rec.Code, rec.Body)
		}
		if uploads.streamID != 7 || uploads.version != model.PRES {
			t.Errorf("reserved stream %d version %s, want 7 PRES", uploads.streamID, uploads.version)
		}
		if gotPath != "/upload/tok" || gotFile != "content" {
			t.Errorf("runner got %q with file %q", gotPath, gotFile)
		}
		if gotCookie != "" {
			t.Errorf("the caller's cookie reached the runner: %q", gotCookie)
		}
	})

	t.Run("passes a runner's refusal on in the gateway's shape", func(t *testing.T) {
		uploads := &fakeUploads{}
		uploads.target = fakeRunner(t, func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "no file in request", http.StatusBadRequest)
		})
		api, m := newMediaAPI(t, uploads)
		expectLecture(m)

		rec := serveUpload(api, uploadRequest(t, path, lectureAdmin, "lecture.mp4", "video/mp4"))
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d: %s", rec.Code, rec.Body)
		}
		if _, msg := gatewayError(t, rec.Body.Bytes()); msg != "runner: no file in request" {
			t.Errorf("message = %q", msg)
		}
	})

	t.Run("a runner that lost the token is worth retrying", func(t *testing.T) {
		uploads := &fakeUploads{}
		uploads.target = fakeRunner(t, func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "unknown or expired upload token", http.StatusNotFound)
		})
		api, m := newMediaAPI(t, uploads)
		expectLecture(m)

		rec := serveUpload(api, uploadRequest(t, path, lectureAdmin, "lecture.mp4", "video/mp4"))
		if rec.Code != http.StatusServiceUnavailable {
			t.Errorf("status = %d: %s", rec.Code, rec.Body)
		}
	})

	t.Run("503s an unreachable runner", func(t *testing.T) {
		// Nothing listens there any more.
		l, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		port := l.Addr().(*net.TCPAddr).Port
		_ = l.Close()
		api, m := newMediaAPI(t, &fakeUploads{target: UploadTarget{Host: "127.0.0.1", Port: port, Token: "tok"}})
		expectLecture(m)

		rec := serveUpload(api, uploadRequest(t, path, lectureAdmin, "lecture.mp4", "video/mp4"))
		if rec.Code != http.StatusServiceUnavailable {
			t.Errorf("status = %d: %s", rec.Code, rec.Body)
		}
	})

	t.Run("503s when no runner can be reserved", func(t *testing.T) {
		api, m := newMediaAPI(t, &fakeUploads{err: errors.New("no runner is available")})
		expectLecture(m)

		rec := serveUpload(api, uploadRequest(t, path, lectureAdmin, "lecture.mp4", "video/mp4"))
		if rec.Code != http.StatusServiceUnavailable {
			t.Errorf("status = %d: %s", rec.Code, rec.Body)
		}
	})

	t.Run("503s without a runner configured", func(t *testing.T) {
		api, m := newMediaAPI(t, nil)
		expectLecture(m)

		rec := serveUpload(api, uploadRequest(t, path, lectureAdmin, "lecture.mp4", "video/mp4"))
		if rec.Code != http.StatusServiceUnavailable {
			t.Errorf("status = %d: %s", rec.Code, rec.Body)
		}
	})

	// No runner is reserved for any refusal below: the fake records it.
	t.Run("401s an anonymous request", func(t *testing.T) {
		uploads := &fakeUploads{}
		api, _ := newMediaAPI(t, uploads)
		rec := serveUpload(api, uploadRequest(t, path, nil, "lecture.mp4", "video/mp4"))
		if rec.Code != http.StatusUnauthorized || uploads.called {
			t.Errorf("status = %d, reserved = %v: %s", rec.Code, uploads.called, rec.Body)
		}
	})

	t.Run("404s a caller who does not administer the course", func(t *testing.T) {
		uploads := &fakeUploads{}
		api, m := newMediaAPI(t, uploads)
		student := &model.User{Model: gorm.Model{ID: 9}, Role: model.StudentType}
		m.courses.EXPECT().GetCourseById(gomock.Any(), uint(1)).Return(courseOne, nil)

		rec := serveUpload(api, uploadRequest(t, path, student, "lecture.mp4", "video/mp4"))
		if rec.Code != http.StatusNotFound || uploads.called {
			t.Errorf("status = %d, reserved = %v: %s", rec.Code, uploads.called, rec.Body)
		}
	})

	t.Run("404s another course's lecture", func(t *testing.T) {
		uploads := &fakeUploads{}
		api, m := newMediaAPI(t, uploads)
		m.courses.EXPECT().GetCourseById(gomock.Any(), uint(1)).Return(courseOne, nil)
		m.streams.EXPECT().GetStreamByID(gomock.Any(), "7").Return(foreignLecture(), nil)

		rec := serveUpload(api, uploadRequest(t, path, lectureAdmin, "lecture.mp4", "video/mp4"))
		if rec.Code != http.StatusNotFound || uploads.called {
			t.Errorf("status = %d, reserved = %v: %s", rec.Code, uploads.called, rec.Body)
		}
	})

	for name, p := range map[string]string{
		"no type":      "/api/v2/courses/1/streams/7/media",
		"unknown type": "/api/v2/courses/1/streams/7/media?type=SCREEN",
	} {
		t.Run("400s "+name, func(t *testing.T) {
			uploads := &fakeUploads{}
			api, m := newMediaAPI(t, uploads)
			expectLecture(m)

			rec := serveUpload(api, uploadRequest(t, p, lectureAdmin, "lecture.mp4", "video/mp4"))
			if rec.Code != http.StatusBadRequest || uploads.called {
				t.Errorf("status = %d, reserved = %v: %s", rec.Code, uploads.called, rec.Body)
			}
		})
	}

	t.Run("409s a lecture that is live", func(t *testing.T) {
		uploads := &fakeUploads{}
		api, m := newMediaAPI(t, uploads)
		live := courseOneLecture()
		live.LiveNow = true
		m.courses.EXPECT().GetCourseById(gomock.Any(), uint(1)).Return(courseOne, nil)
		m.streams.EXPECT().GetStreamByID(gomock.Any(), "7").Return(live, nil)

		rec := serveUpload(api, uploadRequest(t, path, lectureAdmin, "lecture.mp4", "video/mp4"))
		if rec.Code != http.StatusConflict || uploads.called {
			t.Errorf("status = %d, reserved = %v: %s", rec.Code, uploads.called, rec.Body)
		}
	})
}
