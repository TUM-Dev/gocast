package dao

import (
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/TUM-Dev/gocast/model"
)

func setupProgressTestDB(t *testing.T) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:progress_"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	if err := db.AutoMigrate(&model.StreamProgress{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	DB = db
}

func TestSaveProgressWatchedFlag(t *testing.T) {
	setupProgressTestDB(t)
	d := NewProgressDao()
	yes, no := true, false

	stored := func() model.StreamProgress {
		t.Helper()
		rows, err := d.LoadProgress(1, []uint{7})
		if err != nil || len(rows) != 1 {
			t.Fatalf("LoadProgress = %v, %v", rows, err)
		}
		return rows[0]
	}
	save := func(progress float64, watched *bool) {
		t.Helper()
		if err := d.SaveProgress(model.StreamProgress{StreamID: 7, UserID: 1, Progress: progress}, watched); err != nil {
			t.Fatalf("SaveProgress: %v", err)
		}
	}

	save(0.2, nil)
	if p := stored(); p.Progress != 0.2 || p.Watched {
		t.Errorf("first report = %+v, want progress 0.2, not watched", p)
	}

	save(1, &yes)
	if p := stored(); p.Progress != 1 || !p.Watched {
		t.Errorf("marked = %+v, want progress 1, watched", p)
	}

	// A late plain report must not undo the mark.
	save(0.9, nil)
	if p := stored(); p.Progress != 0.9 || !p.Watched {
		t.Errorf("plain report after marking = %+v, want progress 0.9, still watched", p)
	}

	save(0, &no)
	if p := stored(); p.Progress != 0 || p.Watched {
		t.Errorf("unmarked = %+v, want progress 0, not watched", p)
	}
}

func TestSaveProgressCanMarkWatchedWithoutAnEarlierReport(t *testing.T) {
	setupProgressTestDB(t)
	d := NewProgressDao()
	yes := true

	if err := d.SaveProgress(model.StreamProgress{StreamID: 7, UserID: 1, Progress: 1}, &yes); err != nil {
		t.Fatal(err)
	}
	rows, err := d.LoadProgress(1, []uint{7})
	if err != nil || len(rows) != 1 || !rows[0].Watched {
		t.Errorf("LoadProgress = %v, %v; want one watched row", rows, err)
	}
}
