package apiv2

import (
	"encoding/json"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
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
	workers *mock_dao.MockWorkerDao
	keys    *mock_dao.MockUploadKeyDao
}

func newMediaAPI(t *testing.T) (*API, mediaMocks) {
	ctrl := gomock.NewController(t)
	m := mediaMocks{
		streams: mock_dao.NewMockStreamsDao(ctrl),
		courses: mock_dao.NewMockCoursesDao(ctrl),
		workers: mock_dao.NewMockWorkerDao(ctrl),
		keys:    mock_dao.NewMockUploadKeyDao(ctrl),
	}
	return &API{
		dao: dao.DaoWrapper{
			StreamsDao: m.streams, CoursesDao: m.courses, WorkerDao: m.workers, UploadKeyDao: m.keys,
		},
		log: slog.Default(),
	}, m
}

// fakeWorker serves a worker's /upload with handler, and points the proxy at it for
// the test. Its host is the worker's Host.
func fakeWorker(t *testing.T, handler http.HandlerFunc) model.Worker {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	host, port, err := net.SplitHostPort(srv.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	orig := workerUploadPort
	workerUploadPort = port
	t.Cleanup(func() { workerUploadPort = orig })
	return model.Worker{WorkerID: "w1", Host: host}
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

	t.Run("streams the file to the least busy worker with a fresh key", func(t *testing.T) {
		api, m := newMediaAPI(t)
		var gotKey, gotFile, gotCookie string
		idle := fakeWorker(t, func(w http.ResponseWriter, r *http.Request) {
			gotKey, gotCookie = r.URL.Query().Get("key"), r.Header.Get("Cookie")
			if r.URL.Path != "/upload" || r.URL.Query().Has("type") {
				t.Errorf("worker called at %s", r.URL)
			}
			f, _, err := r.FormFile("file")
			if err != nil {
				t.Errorf("worker form: %v", err)
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			b, _ := io.ReadAll(f)
			gotFile = string(b)
		})
		busy := model.Worker{WorkerID: "busy", Host: "busy.invalid", Workload: 5}
		m.courses.EXPECT().GetCourseById(gomock.Any(), uint(1)).Return(courseOne, nil)
		m.streams.EXPECT().GetStreamByID(gomock.Any(), "7").Return(courseOneLecture(), nil)
		m.workers.EXPECT().GetAliveWorkers().Return([]model.Worker{busy, idle})
		var createdKey string
		m.keys.EXPECT().CreateUploadKey(gomock.Any(), uint(7), model.VideoTypePresentation).
			DoAndReturn(func(key string, _ uint, _ model.VideoType) error {
				createdKey = key
				return nil
			})

		req := uploadRequest(t, path, lectureAdmin, "lecture.mp4", "video/mp4")
		req.Header.Set("Cookie", "jwt=secret")
		rec := serveUpload(api, req)
		if rec.Code != http.StatusOK || rec.Body.String() != "{}" {
			t.Fatalf("status = %d: %s", rec.Code, rec.Body)
		}
		if gotKey == "" || gotKey != createdKey || gotFile != "content" {
			t.Errorf("worker got key %q (created %q), file %q", gotKey, createdKey, gotFile)
		}
		if gotCookie != "" {
			t.Errorf("the caller's cookie reached the worker: %q", gotCookie)
		}
	})

	t.Run("passes a worker's refusal on in the gateway's shape", func(t *testing.T) {
		api, m := newMediaAPI(t)
		worker := fakeWorker(t, func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "http: no such file", http.StatusBadRequest)
		})
		m.courses.EXPECT().GetCourseById(gomock.Any(), uint(1)).Return(courseOne, nil)
		m.streams.EXPECT().GetStreamByID(gomock.Any(), "7").Return(courseOneLecture(), nil)
		m.workers.EXPECT().GetAliveWorkers().Return([]model.Worker{worker})
		m.keys.EXPECT().CreateUploadKey(gomock.Any(), uint(7), model.VideoTypePresentation).Return(nil)

		rec := serveUpload(api, uploadRequest(t, path, lectureAdmin, "lecture.mp4", "video/mp4"))
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d: %s", rec.Code, rec.Body)
		}
		if _, msg := gatewayError(t, rec.Body.Bytes()); msg != "worker: http: no such file" {
			t.Errorf("message = %q", msg)
		}
	})

	t.Run("503s an unreachable worker and drops the key", func(t *testing.T) {
		api, m := newMediaAPI(t)
		worker := fakeWorker(t, func(http.ResponseWriter, *http.Request) {})
		// Nothing listens there any more.
		l, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		_, workerUploadPort, _ = net.SplitHostPort(l.Addr().String())
		_ = l.Close()

		m.courses.EXPECT().GetCourseById(gomock.Any(), uint(1)).Return(courseOne, nil)
		m.streams.EXPECT().GetStreamByID(gomock.Any(), "7").Return(courseOneLecture(), nil)
		m.workers.EXPECT().GetAliveWorkers().Return([]model.Worker{worker})
		var createdKey string
		m.keys.EXPECT().CreateUploadKey(gomock.Any(), uint(7), model.VideoTypePresentation).
			DoAndReturn(func(key string, _ uint, _ model.VideoType) error {
				createdKey = key
				return nil
			})
		m.keys.EXPECT().GetUploadKey(gomock.Any()).DoAndReturn(func(key string) (model.UploadKey, error) {
			if key != createdKey {
				t.Errorf("looked up key %q, created %q", key, createdKey)
			}
			return model.UploadKey{Model: gorm.Model{ID: 3}, UploadKey: key}, nil
		})
		m.keys.EXPECT().DeleteUploadKey(gomock.Any()).Return(nil)

		rec := serveUpload(api, uploadRequest(t, path, lectureAdmin, "lecture.mp4", "video/mp4"))
		if rec.Code != http.StatusServiceUnavailable {
			t.Errorf("status = %d: %s", rec.Code, rec.Body)
		}
	})

	// Every refusal below creates no upload key: the mock has no expectation for one.
	t.Run("503s when no worker is alive", func(t *testing.T) {
		api, m := newMediaAPI(t)
		m.courses.EXPECT().GetCourseById(gomock.Any(), uint(1)).Return(courseOne, nil)
		m.streams.EXPECT().GetStreamByID(gomock.Any(), "7").Return(courseOneLecture(), nil)
		m.workers.EXPECT().GetAliveWorkers().Return(nil)

		rec := serveUpload(api, uploadRequest(t, path, lectureAdmin, "lecture.mp4", "video/mp4"))
		if rec.Code != http.StatusServiceUnavailable {
			t.Errorf("status = %d: %s", rec.Code, rec.Body)
		}
	})

	t.Run("401s an anonymous request", func(t *testing.T) {
		api, _ := newMediaAPI(t)
		rec := serveUpload(api, uploadRequest(t, path, nil, "lecture.mp4", "video/mp4"))
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("status = %d: %s", rec.Code, rec.Body)
		}
	})

	t.Run("404s a caller who does not administer the course", func(t *testing.T) {
		api, m := newMediaAPI(t)
		student := &model.User{Model: gorm.Model{ID: 9}, Role: model.StudentType}
		m.courses.EXPECT().GetCourseById(gomock.Any(), uint(1)).Return(courseOne, nil)

		rec := serveUpload(api, uploadRequest(t, path, student, "lecture.mp4", "video/mp4"))
		if rec.Code != http.StatusNotFound {
			t.Errorf("status = %d: %s", rec.Code, rec.Body)
		}
	})

	t.Run("404s another course's lecture", func(t *testing.T) {
		api, m := newMediaAPI(t)
		m.courses.EXPECT().GetCourseById(gomock.Any(), uint(1)).Return(courseOne, nil)
		m.streams.EXPECT().GetStreamByID(gomock.Any(), "7").Return(foreignLecture(), nil)

		rec := serveUpload(api, uploadRequest(t, path, lectureAdmin, "lecture.mp4", "video/mp4"))
		if rec.Code != http.StatusNotFound {
			t.Errorf("status = %d: %s", rec.Code, rec.Body)
		}
	})

	for name, p := range map[string]string{
		"no type":      "/api/v2/courses/1/streams/7/media",
		"unknown type": "/api/v2/courses/1/streams/7/media?type=SCREEN",
	} {
		t.Run("400s "+name, func(t *testing.T) {
			api, m := newMediaAPI(t)
			m.courses.EXPECT().GetCourseById(gomock.Any(), uint(1)).Return(courseOne, nil)
			m.streams.EXPECT().GetStreamByID(gomock.Any(), "7").Return(courseOneLecture(), nil)

			rec := serveUpload(api, uploadRequest(t, p, lectureAdmin, "lecture.mp4", "video/mp4"))
			if rec.Code != http.StatusBadRequest {
				t.Errorf("status = %d: %s", rec.Code, rec.Body)
			}
		})
	}

	t.Run("409s a lecture that is live", func(t *testing.T) {
		api, m := newMediaAPI(t)
		live := courseOneLecture()
		live.LiveNow = true
		m.courses.EXPECT().GetCourseById(gomock.Any(), uint(1)).Return(courseOne, nil)
		m.streams.EXPECT().GetStreamByID(gomock.Any(), "7").Return(live, nil)

		rec := serveUpload(api, uploadRequest(t, path, lectureAdmin, "lecture.mp4", "video/mp4"))
		if rec.Code != http.StatusConflict {
			t.Errorf("status = %d: %s", rec.Code, rec.Body)
		}
	})
}

func TestLeastBusyWorker(t *testing.T) {
	got := leastBusyWorker([]model.Worker{{WorkerID: "a", Workload: 3}, {WorkerID: "b", Workload: 1}, {WorkerID: "c", Workload: 1}})
	if got.WorkerID != "b" {
		t.Errorf("chose %s, want b (the first of the least busy, as v1)", got.WorkerID)
	}
}
