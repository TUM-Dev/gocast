package dao

import (
	"testing"

	"github.com/dgraph-io/ristretto/v2"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/TUM-Dev/gocast/model"
)

// cutStream and cutSection are the columns of model.Stream and model.VideoSection that
// FinishLectureCut reads and writes: SQLite cannot migrate the models themselves, whose
// FULLTEXT indexes are MySQL's.
type cutStream struct {
	gorm.Model
	StartOffset uint
	EndOffset   uint
	Private     bool
}

func (cutStream) TableName() string { return "streams" }

type cutSection struct {
	gorm.Model
	StartHours   uint
	StartMinutes uint
	StartSeconds uint
	StreamID     uint
}

func (cutSection) TableName() string { return "video_sections" }

func setupCutDB(t *testing.T) {
	t.Helper()
	cache, err := ristretto.NewCache[string, any](&ristretto.Config[string, any]{NumCounters: 1e4, MaxCost: 1 << 20, BufferItems: 64})
	if err != nil {
		t.Fatal(err)
	}
	Cache = cache
	db, err := gorm.Open(sqlite.Open("file:cut_"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&cutStream{}, &cutSection{}, &model.Silence{}, &model.LectureCut{}); err != nil {
		t.Fatal(err)
	}
	DB = db
}

func TestFinishLectureCut(t *testing.T) {
	// 10 minutes are cut out, from 20:00 to 30:00 into the recording.
	discard := []model.CutRange{{Start: 1200, End: 1800}}

	seed := func(t *testing.T, streamID uint) {
		t.Helper()
		setupCutDB(t)
		DB.Create(&cutStream{Model: gorm.Model{ID: 1}, Private: false})
		DB.Create(&cutStream{Model: gorm.Model{ID: 2}, StartOffset: 600_000, EndOffset: 3_000_000, Private: true})
		// 10:00 stays, 25:00 is inside the cut, 40:00 moves up to 30:00
		for _, s := range []cutSection{{StartMinutes: 10, StreamID: streamID}, {StartMinutes: 25, StreamID: streamID}, {StartMinutes: 40, StreamID: streamID}, {StartHours: 1, StartSeconds: 5, StreamID: 1}} {
			DB.Create(&s)
		}
		DB.Create(&model.Silence{Start: 1, End: 2, StreamID: streamID})
		DB.Create(&model.LectureCut{TargetStreamID: streamID, SourceStreamID: 1, SourcePrivate: false, Discard: discard})
	}

	t.Run("duplicate", func(t *testing.T) {
		seed(t, 2)
		cut, err := NewLectureCutDao().GetLectureCut(2)
		if err != nil || len(cut.Discard) != 1 || cut.Discard[0] != discard[0] {
			t.Fatalf("GetLectureCut = %+v, %v", cut, err)
		}
		if finished, err := NewLectureCutDao().FinishLectureCut(cut); err != nil || !finished {
			t.Fatalf("FinishLectureCut = %v, %v", finished, err)
		}
		// A second call, as when two versions were ready together, moves nothing again.
		if finished, err := NewLectureCutDao().FinishLectureCut(cut); err != nil || finished {
			t.Fatalf("second FinishLectureCut = %v, %v, want no work", finished, err)
		}

		var target, source cutStream
		DB.First(&target, 2)
		DB.First(&source, 1)
		if target.StartOffset != 600_000 || target.EndOffset != 2_400_000 {
			t.Errorf("offsets = %d, %d, want 600000, 2400000", target.StartOffset, target.EndOffset)
		}
		if target.Private {
			t.Error("the duplicate did not take over the visibility of the source")
		}
		if !source.Private {
			t.Error("the source was not made private")
		}

		var sections []cutSection
		DB.Where("stream_id = ?", 2).Order("id").Find(&sections)
		if len(sections) != 3 || sections[0].StartMinutes != 10 || sections[1].StartMinutes != 20 || sections[2].StartMinutes != 30 {
			t.Errorf("sections = %+v, want 10, 20 and 30 minutes", sections)
		}
		var other cutSection
		DB.Where("stream_id = ?", 1).First(&other)
		if other.StartHours != 1 || other.StartSeconds != 5 {
			t.Errorf("a section of the source moved: %+v", other)
		}

		var silences int64
		DB.Model(&model.Silence{}).Where("stream_id = ?", 2).Count(&silences)
		if silences != 0 {
			t.Errorf("%d silences left", silences)
		}
		if _, err := NewLectureCutDao().GetLectureCut(2); err == nil {
			t.Error("the cut is still open")
		}
	})

	t.Run("a source that was private makes a private duplicate", func(t *testing.T) {
		seed(t, 2)
		cut, _ := NewLectureCutDao().GetLectureCut(2)
		cut.SourcePrivate = true
		if _, err := NewLectureCutDao().FinishLectureCut(cut); err != nil {
			t.Fatal(err)
		}
		var target cutStream
		DB.First(&target, 2)
		if !target.Private {
			t.Error("the duplicate is public")
		}
	})

	t.Run("overwrite leaves visibility alone", func(t *testing.T) {
		seed(t, 1)
		DB.Model(&cutStream{}).Where("id = ?", 1).Updates(map[string]any{"start_offset": 1_500_000, "private": false})
		cut, _ := NewLectureCutDao().GetLectureCut(1)
		cut.SourceStreamID = 1
		if _, err := NewLectureCutDao().FinishLectureCut(cut); err != nil {
			t.Fatal(err)
		}
		var s cutStream
		DB.First(&s, 1)
		if s.Private || s.StartOffset != 1_200_000 {
			t.Errorf("stream = %+v, want public with the offset moved to 1200000", s)
		}
	})
}
