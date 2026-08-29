package helpers

import (
	"database/sql"
	"testing"
	"time"

	"gorm.io/gorm"

	"github.com/TUM-Dev/gocast/model"
)

// The start page lists a course by the date of its most recent recording and its next
// lecture, so both travel with the course rather than as ids to fetch them with. What
// is derived is derived for the caller: a private lecture is not one of them unless
// the caller administers the course.
func TestParseCourseToProtoDerivedStreams(t *testing.T) {
	now := time.Now()

	// Ascending by start, which GetLastRecording assumes.
	streams := []model.Stream{
		{Model: gorm.Model{ID: 1}, Start: now.Add(-48 * time.Hour), End: now.Add(-47 * time.Hour), Recording: true},
		{Model: gorm.Model{ID: 2}, Start: now.Add(-24 * time.Hour), End: now.Add(-23 * time.Hour), Recording: true, Private: true},
		{Model: gorm.Model{ID: 3}, Start: now.Add(24 * time.Hour), End: now.Add(25 * time.Hour), Private: true},
		{Model: gorm.Model{ID: 4}, Start: now.Add(48 * time.Hour), End: now.Add(49 * time.Hour)},
	}

	course := model.Course{
		Model: gorm.Model{ID: 1}, Name: "Course", Slug: "course",
		Visibility: "public", UserID: 42, Streams: streams,
	}

	student := &model.User{Model: gorm.Model{ID: 7}, Role: model.StudentType}
	// Owns the course, so the private lectures are theirs to see.
	owner := &model.User{Model: gorm.Model{ID: 42}, Role: model.LecturerType}

	tests := []struct {
		name              string
		user              *model.User
		wantLastRecording uint32
		wantNextLecture   uint32
	}{
		{"anonymous skips the private lectures", nil, 1, 4},
		{"student skips the private lectures", student, 1, 4},
		{"course owner sees them", owner, 2, 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseCourseToProto(course, tt.user)

			if got.LastRecording == nil {
				t.Fatalf("last recording is absent, want stream %d", tt.wantLastRecording)
			}
			if got.LastRecording.Id != tt.wantLastRecording {
				t.Errorf("last recording = %d, want %d", got.LastRecording.Id, tt.wantLastRecording)
			}
			if got.NextLecture == nil {
				t.Fatalf("next lecture is absent, want stream %d", tt.wantNextLecture)
			}
			if got.NextLecture.Id != tt.wantNextLecture {
				t.Errorf("next lecture = %d, want %d", got.NextLecture.Id, tt.wantNextLecture)
			}
		})
	}
}

// Both getters answer with a zero-value stream rather than nothing, and the v1 page
// tested every one for `ID !== 0`. Absent says the same thing without the trap.
func TestParseCourseToProtoOmitsStreamsThatDoNotExist(t *testing.T) {
	empty := model.Course{Model: gorm.Model{ID: 1}, Slug: "empty", Visibility: "public", UserID: 42}

	got := ParseCourseToProto(empty, nil)

	if got.LastRecording != nil {
		t.Errorf("last recording = %v, want absent", got.LastRecording)
	}
	if got.NextLecture != nil {
		t.Errorf("next lecture = %v, want absent", got.NextLecture)
	}
}

// Pinned and IsAdmin describe the caller's relationship to the course, so the same
// course parsed for two callers must not answer the same way.
func TestParseCourseToProtoCallerRelationship(t *testing.T) {
	course := model.Course{Model: gorm.Model{ID: 1}, Slug: "course", Visibility: "public", UserID: 42}
	other := model.Course{Model: gorm.Model{ID: 2}, Slug: "other", Visibility: "public", UserID: 42}

	pinner := &model.User{Model: gorm.Model{ID: 7}, Role: model.StudentType, PinnedCourses: []model.Course{course}}
	owner := &model.User{Model: gorm.Model{ID: 42}, Role: model.LecturerType}

	tests := []struct {
		name        string
		course      model.Course
		user        *model.User
		wantPinned  bool
		wantIsAdmin bool
	}{
		{"anonymous", course, nil, false, false},
		{"pinned by this caller", course, pinner, true, false},
		{"a course they did not pin", other, pinner, false, false},
		{"owner administers it", course, owner, false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseCourseToProto(tt.course, tt.user)

			if got.Pinned != tt.wantPinned {
				t.Errorf("pinned = %v, want %v", got.Pinned, tt.wantPinned)
			}
			if got.IsAdmin != tt.wantIsAdmin {
				t.Errorf("isAdmin = %v, want %v", got.IsAdmin, tt.wantIsAdmin)
			}
			if got.Visibility != tt.course.Visibility {
				t.Errorf("visibility = %q, want %q", got.Visibility, tt.course.Visibility)
			}
		})
	}
}

// The duration column is null until a recording has been processed, and a listing
// that shows no running time reads as a broken lecture rather than an unprocessed one.
// v1 answered with the scheduled length in the meantime — model.Stream.ToDTO.
func TestParseStreamToProtoDuration(t *testing.T) {
	course := model.Course{Model: gorm.Model{ID: 1}, Visibility: "public", UserID: 42}
	start := time.Date(2026, 4, 11, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name   string
		stream model.Stream
		want   uint32
	}{
		{
			"recorded, so the measured length",
			model.Stream{Start: start, End: start.Add(time.Hour), Duration: sql.NullInt32{Int32: 596, Valid: true}},
			596,
		},
		{
			"not processed yet, so the scheduled length",
			model.Stream{Start: start, End: start.Add(10 * time.Minute)},
			600,
		},
		{
			// A lecture with no times at all would otherwise report a negative length.
			"no times at all",
			model.Stream{},
			0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ParseStreamToProto(tt.stream, course, nil).Duration; got != tt.want {
				t.Errorf("duration = %d, want %d", got, tt.want)
			}
		})
	}
}

// Only a course administrator is ever sent a private stream, so the flag marks the
// lectures the rest of the course cannot see.
func TestParseStreamToProtoIsPubliclyVisible(t *testing.T) {
	course := model.Course{Model: gorm.Model{ID: 1}, Visibility: "public", UserID: 42}

	if got := ParseStreamToProto(model.Stream{Model: gorm.Model{ID: 1}}, course, nil); !got.IsPubliclyVisible {
		t.Error("a normal stream reported as not publicly visible")
	}
	if got := ParseStreamToProto(model.Stream{Model: gorm.Model{ID: 2}, Private: true}, course, nil); got.IsPubliclyVisible {
		t.Error("a private stream reported as publicly visible")
	}
}
