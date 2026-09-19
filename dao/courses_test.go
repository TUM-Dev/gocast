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

	// Correlated against the course being filtered rather than grouped over the whole
	// table. Grouped, the aggregate reads every recording ever made on each listing,
	// because the course_id the listing asks for is applied to the outer query only --
	// which is the cost this filter exists to avoid.
	if strings.Contains(sql, "GROUP BY") {
		t.Errorf("the latest recording is grouped over the table instead of looked up per course:\n%s", sql)
	}
	if !strings.Contains(sql, "latest.course_id = streams.course_id") {
		t.Errorf("the latest recording is not correlated to the course being filtered:\n%s", sql)
	}
	// Aliased, because an unaliased `streams` inside the subquery names the subquery's
	// own table rather than the outer row, and the condition would then yield the
	// latest recording of any course instead of this one's.
	if !strings.Contains(sql, "streams AS latest") {
		t.Errorf("the inner table is not aliased, so it names itself rather than the outer row:\n%s", sql)
	}
	// Per privacy, because a course administrator is shown its private lectures and
	// everyone else is not: the latest public recording has to outlive a private one
	// recorded after it.
	if !strings.Contains(sql, "latest.private = streams.private") {
		t.Errorf("the latest recording is not matched per privacy:\n%s", sql)
	}
	// By end, not by start: a lecture that is running right now is the next one there
	// is, and GetNextLecture picks it by its end.
	if !strings.Contains(sql, "end > NOW()") {
		t.Errorf("a lecture that has started but not ended is left out:\n%s", sql)
	}
	// And only the earliest of them, per privacy for the same reason the recording is:
	// a bare `end > NOW()` keeps the whole rest of the term, while a single MIN over
	// the course would let a private lecture take the row and leave everyone but a
	// course administrator with no next lecture at all.
	if !strings.Contains(sql, "MIN(upcoming.start)") {
		t.Errorf("every lecture still to come is kept, not just the earliest:\n%s", sql)
	}
	if !strings.Contains(sql, "upcoming.private = streams.private") {
		t.Errorf("the next lecture is not matched per privacy:\n%s", sql)
	}
	// GetLastRecording and GetNextLecture both walk the lectures in order.
	if !strings.Contains(sql, "ORDER BY start asc") {
		t.Errorf("the lectures are not ordered by start:\n%s", sql)
	}
}
