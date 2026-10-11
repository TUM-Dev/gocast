package api

import (
	"context"
	"strings"
	"testing"

	"github.com/meilisearch/meilisearch-go"

	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/model"
)

// recordingMeili keeps what it was asked and answers with no hits.
type recordingMeili struct {
	searchType                                 int
	courseFilter, streamFilter, subtitleFilter string
	answer                                     *meilisearch.MultiSearchResponse
}

func (m *recordingMeili) SearchSubtitles(string, uint) *meilisearch.SearchResponse { return nil }

func (m *recordingMeili) Search(_ string, _ int64, searchType int, courseFilter, streamFilter, subtitleFilter string) *meilisearch.MultiSearchResponse {
	m.searchType, m.courseFilter, m.streamFilter, m.subtitleFilter = searchType, courseFilter, streamFilter, subtitleFilter
	if m.answer != nil {
		return m.answer
	}
	return &meilisearch.MultiSearchResponse{}
}

// The catalog chooses v1's search shapes from the scope: which indexes, and the
// filters an anonymous caller gets.
func TestSearchCatalogScopes(t *testing.T) {
	anonymous := (*model.User)(nil)
	s2022 := model.Semester{Year: 2022, TeachingTerm: "S"}

	t.Run("no scope: courses of every semester, public only for anonymous", func(t *testing.T) {
		m := &recordingMeili{}
		_, err := SearchCatalog{Dao: dao.DaoWrapper{}, Meili: m}.Search(context.Background(), anonymous, "bier", 10, SearchScope{})
		if err != nil {
			t.Fatalf("Search: %v", err)
		}
		if m.searchType != 4 || !strings.Contains(m.courseFilter, `visibility = "public"`) || m.streamFilter != "" {
			t.Errorf("asked type %d course %q stream %q", m.searchType, m.courseFilter, m.streamFilter)
		}
	})

	t.Run("one semester: courses and lectures of it", func(t *testing.T) {
		m := &recordingMeili{}
		_, err := SearchCatalog{Dao: dao.DaoWrapper{}, Meili: m}.Search(context.Background(), anonymous, "bier", 10, SearchScope{Semesters: []model.Semester{s2022}})
		if err != nil {
			t.Fatalf("Search: %v", err)
		}
		if m.searchType != 6 || !strings.Contains(m.streamFilter, `year = 2022 AND semester = "S"`) {
			t.Errorf("asked type %d stream %q", m.searchType, m.streamFilter)
		}
	})

	t.Run("one semester, courses only: no lectures", func(t *testing.T) {
		m := &recordingMeili{}
		_, err := SearchCatalog{Dao: dao.DaoWrapper{}, Meili: m}.Search(context.Background(), anonymous, "bier", 10,
			SearchScope{Semesters: []model.Semester{s2022}, CoursesOnly: true})
		if err != nil {
			t.Fatalf("Search: %v", err)
		}
		if m.searchType != 4 || m.streamFilter != "" {
			t.Errorf("asked type %d stream %q", m.searchType, m.streamFilter)
		}
	})

	t.Run("inside courses: their lectures and subtitles, private lectures left out", func(t *testing.T) {
		m := &recordingMeili{}
		course := model.Course{Streams: []model.Stream{{}, {Private: true}}}
		course.ID, course.Streams[0].ID, course.Streams[1].ID = 1, 7, 8
		_, err := SearchCatalog{Dao: dao.DaoWrapper{}, Meili: m}.Search(context.Background(), anonymous, "bier", 10, SearchScope{Courses: []model.Course{course}})
		if err != nil {
			t.Fatalf("Search: %v", err)
		}
		if m.searchType != 3 || m.courseFilter != "" || !strings.Contains(m.streamFilter, "courseID IN [1] AND private = 0") || m.subtitleFilter != "streamID IN [7]" {
			t.Errorf("asked type %d course %q stream %q subtitles %q", m.searchType, m.courseFilter, m.streamFilter, m.subtitleFilter)
		}
	})

	t.Run("no Meilisearch, no answer", func(t *testing.T) {
		if _, err := (SearchCatalog{}).Search(context.Background(), anonymous, "bier", 10, SearchScope{}); err != ErrSearchUnavailable {
			t.Errorf("err = %v", err)
		}
		if _, err := (SearchCatalog{Meili: nilMeili{}}).Search(context.Background(), anonymous, "bier", 10, SearchScope{}); err != ErrSearchUnavailable {
			t.Errorf("err = %v", err)
		}
	})
}

// nilMeili is a Meilisearch that did not answer, as the client reports it.
type nilMeili struct{}

func (nilMeili) SearchSubtitles(string, uint) *meilisearch.SearchResponse { return nil }
func (nilMeili) Search(string, int64, int, string, string, string) *meilisearch.MultiSearchResponse {
	return nil
}

func TestParseKeys(t *testing.T) {
	semesters, err := ParseSemesterKeys([]string{"2022S", "2021W"})
	if err != nil || len(semesters) != 2 || semesters[1].Year != 2021 || semesters[1].TeachingTerm != "W" {
		t.Errorf("semesters = %v, %v", semesters, err)
	}
	if _, err := ParseSemesterKeys([]string{"2022X"}); err == nil {
		t.Error("2022X parsed")
	}
	if got, err := ParseSemesterKeys(nil); got != nil || err != nil {
		t.Errorf("nil keys = %v, %v", got, err)
	}
	if _, err := ParseCourseKeys(context.Background(), dao.DaoWrapper{}, []string{"nonsense"}); err != ErrUnknownCourse {
		t.Errorf("malformed course key: %v", err)
	}
}
