package tools

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/meilisearch/meilisearch-go"
	"go.uber.org/mock/gomock"

	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/mock_dao"
)

// Export used to hand AddDocumentsInBatches a *[]MeiliCourse; the client reflects over
// that value to slice it into batches, so a pointer panicked and took the whole export
// down before a single course reached meili.
func TestMeiliExporterExportsCoursesInBatches(t *testing.T) {
	const courseCount = 501 // two batches at a batch size of 500

	server, batches := meiliStub(t)

	ctrl := gomock.NewController(t)
	coursesDao := mock_dao.NewMockCoursesDao(ctrl)
	coursesDao.EXPECT().ExecAllCourses(gomock.Any()).Do(func(f func([]dao.Course)) {
		courses := make([]dao.Course, courseCount)
		for i := range courses {
			courses[i] = dao.Course{ID: uint(i + 1), Name: "Course", Slug: "c"}
		}
		f(courses)
	})
	streamsDao := mock_dao.NewMockStreamsDao(ctrl)
	streamsDao.EXPECT().ExecAllStreamsWithCoursesAndSubtitlesBatched(gomock.Any())

	exporter := &MeiliExporter{
		c: meilisearch.New(server.URL),
		d: dao.DaoWrapper{CoursesDao: coursesDao, StreamsDao: streamsDao},
	}

	exporter.Export()

	got := batches("COURSES")
	if len(got) != 2 {
		t.Fatalf("course batches = %d, want 2", len(got))
	}
	if len(got[0]) != 500 || len(got[1]) != 1 {
		t.Fatalf("batch sizes = %d, %d, want 500, 1", len(got[0]), len(got[1]))
	}
	if got[0][0].ID != 1 || got[1][0].ID != courseCount {
		t.Errorf("first/last exported ID = %d/%d, want 1/%d", got[0][0].ID, got[1][0].ID, courseCount)
	}
}

// meiliStub serves the handful of endpoints Export calls and records the documents
// posted per index, in order.
func meiliStub(t *testing.T) (*httptest.Server, func(index string) [][]MeiliCourse) {
	t.Helper()

	var mu sync.Mutex
	posted := map[string][][]MeiliCourse{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Errorf("reading %s: %v", r.URL.Path, err)
			}
			var docs []MeiliCourse
			if err := json.Unmarshal(body, &docs); err != nil {
				t.Errorf("unmarshalling documents posted to %s: %v", r.URL.Path, err)
			}
			// paths look like /indexes/COURSES/documents
			index := ""
			if parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/"); len(parts) > 1 {
				index = parts[1]
			}
			mu.Lock()
			posted[index] = append(posted[index], docs)
			mu.Unlock()
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"taskUid":1,"indexUid":"","status":"enqueued","type":"documentAdditionOrUpdate","enqueuedAt":"2024-01-01T00:00:00Z"}`))
	}))
	t.Cleanup(server.Close)

	return server, func(index string) [][]MeiliCourse {
		mu.Lock()
		defer mu.Unlock()
		return posted[index]
	}
}
