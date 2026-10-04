package dao

import (
	"testing"
	"time"

	"github.com/dgraph-io/ristretto/v2"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// seriesStream is the part of model.Stream the series queries touch. SQLite cannot
// migrate the model itself, whose FULLTEXT indexes are MySQL's.
type seriesStream struct {
	gorm.Model
	CourseID         uint
	SeriesIdentifier string
	Name             string
	Description      string
	Start            time.Time
	End              time.Time
	ChatEnabled      bool
	LectureHallID    *uint
}

func (seriesStream) TableName() string { return "streams" }

// setupSeriesTestDB holds a series "s1" of three lectures in course 1 (streams 1-3,
// Mondays 10:00-11:30 UTC), a copy of one of them in course 2 that kept the series
// identifier (stream 4), and an unrelated lecture in course 1 (stream 5).
func setupSeriesTestDB(t *testing.T) {
	t.Helper()

	cache, err := ristretto.NewCache[string, any](&ristretto.Config[string, any]{
		NumCounters: 1e4, MaxCost: 1 << 20, BufferItems: 64,
	})
	if err != nil {
		t.Fatalf("create cache: %v", err)
	}
	Cache = cache

	db, err := gorm.Open(sqlite.Open("file:series_"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	if err := db.AutoMigrate(&seriesStream{}); err != nil {
		t.Fatalf("migrate stream model: %v", err)
	}
	DB = db

	monday := time.Date(2026, 10, 5, 10, 0, 0, 0, time.UTC)
	week := 7 * 24 * time.Hour
	streams := []seriesStream{
		{Model: gorm.Model{ID: 1}, CourseID: 1, SeriesIdentifier: "s1", Name: "A", Start: monday, End: monday.Add(90 * time.Minute)},
		{Model: gorm.Model{ID: 2}, CourseID: 1, SeriesIdentifier: "s1", Name: "A", Start: monday.Add(week), End: monday.Add(week + 90*time.Minute)},
		{Model: gorm.Model{ID: 3}, CourseID: 1, SeriesIdentifier: "s1", Name: "A", Start: monday.Add(2 * week), End: monday.Add(2*week + 90*time.Minute)},
		{Model: gorm.Model{ID: 4}, CourseID: 2, SeriesIdentifier: "s1", Name: "A", Start: monday, End: monday.Add(90 * time.Minute)},
		{Model: gorm.Model{ID: 5}, CourseID: 1, SeriesIdentifier: "other", Name: "A", Start: monday, End: monday.Add(90 * time.Minute)},
	}
	for i := range streams {
		if err := DB.Create(&streams[i]).Error; err != nil {
			t.Fatalf("create stream: %v", err)
		}
	}
}

func loadStream(t *testing.T, id uint) seriesStream {
	t.Helper()
	var s seriesStream
	if err := DB.Unscoped().First(&s, id).Error; err != nil {
		t.Fatalf("load stream %d: %v", id, err)
	}
	return s
}

func TestUpdateCourseLectureSeries(t *testing.T) {
	setupSeriesTestDB(t)

	name, chat, hall := "B", true, uint(0)
	err := NewStreamsDao().UpdateCourseLectureSeries(1, "s1", LectureSeriesUpdate{Name: &name, ChatEnabled: &chat, LectureHallID: &hall})
	if err != nil {
		t.Fatalf("UpdateCourseLectureSeries: %v", err)
	}

	for _, id := range []uint{1, 2, 3} {
		if s := loadStream(t, id); s.Name != "B" || !s.ChatEnabled {
			t.Errorf("stream %d = %q chat %v, want the update", id, s.Name, s.ChatEnabled)
		}
	}
	for _, id := range []uint{4, 5} {
		if s := loadStream(t, id); s.Name != "A" || s.ChatEnabled {
			t.Errorf("stream %d outside the course's series changed: %q chat %v", id, s.Name, s.ChatEnabled)
		}
	}
}

func TestUpdateCourseLectureSeriesTime(t *testing.T) {
	setupSeriesTestDB(t)

	// Stream 2 moves to Tuesday 14:00-15:00; the others keep their dates.
	start := time.Date(2026, 10, 13, 14, 0, 0, 0, time.UTC)
	if err := NewStreamsDao().UpdateCourseLectureSeriesTime(1, 2, "s1", start, start.Add(time.Hour)); err != nil {
		t.Fatalf("UpdateCourseLectureSeriesTime: %v", err)
	}

	if s := loadStream(t, 2); !s.Start.Equal(start) || !s.End.Equal(start.Add(time.Hour)) {
		t.Errorf("stream 2 = %v-%v, want the times given", s.Start, s.End)
	}
	for id, day := range map[uint]int{1: 5, 3: 19} {
		s := loadStream(t, id).Start.UTC()
		if s.Day() != day || s.Hour() != 14 || s.Minute() != 0 {
			t.Errorf("stream %d starts %v, want 14:00 on the %dth", id, s, day)
		}
		if d := loadStream(t, id).End.Sub(loadStream(t, id).Start); d != time.Hour {
			t.Errorf("stream %d lasts %v, want 1h", id, d)
		}
	}
	for _, id := range []uint{4, 5} {
		if s := loadStream(t, id).Start.UTC(); s.Hour() != 10 {
			t.Errorf("stream %d outside the course's series moved to %v", id, s)
		}
	}
}

func TestDeleteCourseLectureSeries(t *testing.T) {
	setupSeriesTestDB(t)

	if err := NewStreamsDao().DeleteCourseLectureSeries(1, "s1"); err != nil {
		t.Fatalf("DeleteCourseLectureSeries: %v", err)
	}
	for _, id := range []uint{1, 2, 3} {
		if !loadStream(t, id).DeletedAt.Valid {
			t.Errorf("stream %d was not deleted", id)
		}
	}
	for _, id := range []uint{4, 5} {
		if loadStream(t, id).DeletedAt.Valid {
			t.Errorf("stream %d outside the course's series was deleted", id)
		}
	}
}
