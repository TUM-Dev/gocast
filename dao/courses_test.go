package dao

import (
	"strings"
	"testing"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"github.com/TUM-Dev/gocast/model"
)

// The listing query preloads only the lectures a listing derives anything from, which
// is the difference between two rows per course and a semester's worth of them. Getting
// the condition wrong empties the start page rather than failing loudly, so the SQL it
// builds is asserted here; frontend/e2e/visibility.spec.ts covers the page itself.
func TestPublicCourseStreamFilter(t *testing.T) {
	db, err := gorm.Open(
		mysql.New(mysql.Config{DriverName: "mysql", DSN: "", SkipInitializeWithVersion: true}),
		&gorm.Config{DryRun: true, DisableAutomaticPing: true},
	)
	if err != nil {
		t.Fatalf("opening a dry-run connection: %v", err)
	}

	previous := DB
	DB = db
	t.Cleanup(func() { DB = previous })

	var streams []model.Stream
	sql := publicCourseStreamFilter(db.Session(&gorm.Session{})).Find(&streams).Statement.SQL.String()

	// The latest recording is grouped by course, not by the whole table: a subquery
	// over `streams` cannot name the `streams` row being filtered, so a correlated
	// condition compares the inner table against itself and leaves every course but
	// the one holding the newest recording of all with no recording at all.
	if !strings.Contains(sql, "GROUP BY course_id, private") {
		t.Errorf("the latest recording is not grouped per course and privacy:\n%s", sql)
	}
	// Per privacy, because a course administrator is shown its private lectures and
	// everyone else is not: the latest public recording has to outlive a private one
	// recorded after it.
	if !strings.Contains(sql, "(course_id, private, start) IN") {
		t.Errorf("the latest recording is not matched per course and privacy:\n%s", sql)
	}
	// By end, not by start: a lecture that is running right now is the next one there
	// is, and GetNextLecture picks it by its end.
	if !strings.Contains(sql, "end > NOW()") {
		t.Errorf("a lecture that has started but not ended is left out:\n%s", sql)
	}
	// GetLastRecording and GetNextLecture both walk the lectures in order.
	if !strings.Contains(sql, "ORDER BY start asc") {
		t.Errorf("the lectures are not ordered by start:\n%s", sql)
	}
}
