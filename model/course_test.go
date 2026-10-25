package model

import (
	"database/sql"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
t"gorm.io/gorm"
)

func TestShouldGenerateSubtitles(t *testing.T) {
	// Test case 1: No language set
	t.Run("NoLanguageSet", func(t *testing.T) {
		course := Course{
			Language: sql.NullString{Valid: false},
		}
		assert.False(t, course.ShouldGenerateSubtitles(COMB, 0))
		assert.False(t, course.ShouldGenerateSubtitles(PRES, 1))
		assert.False(t, course.ShouldGenerateSubtitles(CAM, 1))
	})

	// Test case 2: Self-stream (lectureHallID == 0)
	t.Run("SelfStream", func(t *testing.T) {
		course := Course{
			Language: sql.NullString{String: "en", Valid: true},
		}
		assert.True(t, course.ShouldGenerateSubtitles(COMB, 0))
		assert.False(t, course.ShouldGenerateSubtitles(PRES, 0))
		assert.False(t, course.ShouldGenerateSubtitles(CAM, 0))
	})

	// Test case 3: Lecture hall stream - SourceModeCAMOnly
	t.Run("LectureHall_CAMOnly", func(t *testing.T) {
		pref := []SourcePreference{{LectureHallID: 1, SourceMode: SourceModeCAMOnly}}
		prefBytes, _ := json.Marshal(pref)
		course := Course{
			Language:          sql.NullString{String: "en", Valid: true},
			SourcePreferences: string(prefBytes),
		}
		assert.False(t, course.ShouldGenerateSubtitles(COMB, 1))
		assert.False(t, course.ShouldGenerateSubtitles(PRES, 1))
		assert.True(t, course.ShouldGenerateSubtitles(CAM, 1))
	})

	// Test case 4: Lecture hall stream - SourceModePRESOnly
	t.Run("LectureHall_PRESOnly", func(t *testing.T) {
		pref := []SourcePreference{{LectureHallID: 1, SourceMode: SourceModePRESOnly}}
		prefBytes, _ := json.Marshal(pref)
		course := Course{
			Language:          sql.NullString{String: "en", Valid: true},
			SourcePreferences: string(prefBytes),
		}
		assert.False(t, course.ShouldGenerateSubtitles(COMB, 1))
		assert.True(t, course.ShouldGenerateSubtitles(PRES, 1))
		assert.False(t, course.ShouldGenerateSubtitles(CAM, 1))
	})

	// Test case 5: Lecture hall stream - SourceModeCOMB (default behavior if preference exists but not CAM/PRES only)
	t.Run("LectureHall_COMB", func(t *testing.T) {
		pref := []SourcePreference{{LectureHallID: 1, SourceMode: SourceModeCOMB}}
		prefBytes, _ := json.Marshal(pref)
		course := Course{
			Language:          sql.NullString{String: "en", Valid: true},
			SourcePreferences: string(prefBytes),
		}
		assert.False(t, course.ShouldGenerateSubtitles(COMB, 1))
		assert.True(t, course.ShouldGenerateSubtitles(PRES, 1)) // Default to PRES if COMB is preferred
		assert.False(t, course.ShouldGenerateSubtitles(CAM, 1))
	})

	// Test case 6: Lecture hall stream - No specific source preference for the given lecture hall
	t.Run("LectureHall_NoSpecificPreference", func(t *testing.T) {
		pref := []SourcePreference{{LectureHallID: 99, SourceMode: SourceModeCAMOnly}} // Preference for another LH
		prefBytes, _ := json.Marshal(pref)
		course := Course{
			Language:          sql.NullString{String: "en", Valid: true},
			SourcePreferences: string(prefBytes),
		}
		assert.False(t, course.ShouldGenerateSubtitles(COMB, 1))
		assert.True(t, course.ShouldGenerateSubtitles(PRES, 1)) // Default to PRES
		assert.False(t, course.ShouldGenerateSubtitles(CAM, 1))
	})

	// Test case 7: Lecture hall stream - Empty SourcePreferences string
	t.Run("LectureHall_EmptySourcePreferences", func(t *testing.T) {
		course := Course{
			Language:          sql.NullString{String: "en", Valid: true},
			SourcePreferences: "", // Empty string, GetSourcePreference will return empty slice
		}
		assert.False(t, course.ShouldGenerateSubtitles(COMB, 1))
		assert.True(t, course.ShouldGenerateSubtitles(PRES, 1)) // Default to PRES
		assert.False(t, course.ShouldGenerateSubtitles(CAM, 1))
	})

	// Test case 8: Lecture hall stream - Invalid SourcePreferences string
	t.Run("LectureHall_InvalidSourcePreferences", func(t *testing.T) {
		course := Course{
			Language:          sql.NullString{String: "en", Valid: true},
			SourcePreferences: "invalid json", // Invalid JSON, GetSourcePreference will return empty slice
		}
		assert.False(t, course.ShouldGenerateSubtitles(COMB, 1))
		assert.True(t, course.ShouldGenerateSubtitles(PRES, 1)) // Default to PRES
		assert.False(t, course.ShouldGenerateSubtitles(CAM, 1))
	})
}

func TestValidateCourseName(t *testing.T) {
	valid := []string{
		"Funktionale Programmierung und Verifikation (IN0003)",
		"Analysis I/II",
		"Einführung in die Informatik",
		"../../escaped", // path-like, but harmless once sanitized for filesystem use
	}
	for _, name := range valid {
		if err := validateCourseName(name); err != nil {
			t.Errorf("validateCourseName(%q) = %v, want nil", name, err)
		}
	}

	invalid := map[string]string{
		"empty":      "",
		"whitespace": "   ",
		"dot":        ".",
		"dotdot":     "..",
		"nul byte":   "course\x00name",
		"newline":    "course\nname",
		"too long":   strings.Repeat("a", 256),
	}
	for label, name := range invalid {
		if err := validateCourseName(name); err == nil {
			t.Errorf("validateCourseName(%s) = nil, want error", label)
		}
	}
}

func TestUniqueCourses(t *testing.T) {
	a, b, c := Course{Model: gorm.Model{ID: 1}, Name: "first"}, Course{Model: gorm.Model{ID: 2}}, Course{Model: gorm.Model{ID: 1}, Name: "dup"}
	got := UniqueCourses([]Course{a, b, c})
	if len(got) != 2 || got[0].Name != "first" || got[1].ID != 2 {
		t.Errorf("UniqueCourses kept %+v, want the first course 1 then course 2", got)
	}
	if got := UniqueCourses(nil); got == nil || len(got) != 0 {
		t.Errorf("UniqueCourses(nil) = %#v, want an empty non-nil slice", got)
	}
}
