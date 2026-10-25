package migrations

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

var (
	t0       = time.Date(2025, 10, 20, 9, 0, 0, 0, time.UTC)
	semester = func(id uint, name, slug string, year int, term string, created time.Time) auditCourse {
		return auditCourse{ID: id, Name: name, Slug: slug, Year: year, TeachingTerm: term, CreatedAt: created}
	}
)

func resolverFixture() *auditResolver {
	deleted := semester(3, "Analysis 1", "ana1", 2024, "W", t0.AddDate(-1, 0, 0))
	deleted.DeletedAt = gorm.DeletedAt{Time: t0.AddDate(0, -6, 0), Valid: true}
	return newAuditResolver(
		[]auditCourse{
			deleted,
			semester(4, "Analysis 1", "ana1", 2025, "W", t0),
			semester(5, "Brauereiwesen", "brau", 2025, "W", t0),
			semester(6, "Brauereiwesen", "brau", 2024, "W", t0.AddDate(-1, 0, 0)),
		},
		[]auditStream{
			{ID: 100, CourseID: 4, CreatedAt: t0.Add(time.Minute)},
			{ID: 101, CourseID: 4, CreatedAt: t0.Add(time.Minute + time.Second)},
			{ID: 102, CourseID: 5, CreatedAt: t0.Add(time.Hour)},
			{ID: 103, CourseID: 6, CreatedAt: t0.AddDate(-1, 0, 0).Add(time.Hour)},
		},
	)
}

func TestAuditResolver(t *testing.T) {
	r := resolverFixture()
	tests := []struct {
		name string
		row  auditRow
		want auditLink
		ok   bool
	}{
		{"stream edit names the stream", auditRow{Type: auditStreamEdit, Message: "100: (Visibility: true)"}, auditLink{4, 100}, true},
		{"stream edit of an unknown stream", auditRow{Type: auditStreamEdit, Message: "999: ended (discard: false)"}, auditLink{}, false},
		{"stream deletion names the stream last", auditRow{Type: auditStreamDelete, Message: "'Analysis 1': 2025 21 Oct, 15:45 (102)"}, auditLink{5, 102}, true},
		{"series deletion", auditRow{Type: auditStreamDelete, Message: "'Analysis 1': 2025 21 Oct, 15:45 (103 and series)"}, auditLink{6, 103}, true},
		{"course deletion names the course", auditRow{Type: auditCourseDelete, Message: "'Analysis 1' (2024, W)[3]. Token: abc"}, auditLink{courseID: 3}, true},
		{"course deletion of an unknown course", auditRow{Type: auditCourseDelete, Message: "'x' (2024, W)[77]"}, auditLink{}, false},
		{
			"stream creation among streams written at that moment",
			auditRow{Type: auditStreamCreate, Message: "Stream for 'Brauereiwesen' Created. Time: 2025 20 Oct, 12:00", CreatedAt: t0.Add(time.Hour)},
			auditLink{5, 102}, true,
		},
		{
			"stream creation of a batch names the course only",
			auditRow{Type: auditStreamCreate, Message: "Stream for 'Analysis 1' Created. Time: 2025 20 Oct, 12:00", CreatedAt: t0.Add(time.Minute)},
			auditLink{courseID: 4}, true,
		},
		{
			"stream creation matching no stream written then",
			auditRow{Type: auditStreamCreate, Message: "Stream for 'Brauereiwesen' Created. Time: x", CreatedAt: t0.Add(48 * time.Hour)},
			auditLink{}, false,
		},
		{
			"course creation is told apart from last semester's by when it happened",
			auditRow{Type: auditCourseCreate, Message: "brau:'Brauereiwesen' (2025, W)", CreatedAt: t0.Add(2 * time.Second)},
			auditLink{courseID: 5}, true,
		},
		{
			"course creation too long after the course",
			auditRow{Type: auditCourseCreate, Message: "brau:'Brauereiwesen' (2025, W)", CreatedAt: t0.Add(time.Hour)},
			auditLink{}, false,
		},
		{
			"an opt-in by token among the courses alive then",
			auditRow{Type: auditCourseCreate, Message: "opted in by token, Analysis 1:'ana1'", CreatedAt: t0.AddDate(0, 1, 0)},
			auditLink{courseID: 4}, true,
		},
		{
			"a course edit while two courses of that name were alive is left alone",
			auditRow{Type: auditCourseEdit, Message: "Brauereiwesen:'brau' add: Anja (1)", CreatedAt: t0.AddDate(0, 1, 0)},
			auditLink{}, false,
		},
		{
			"a course edit before the newer course existed is the older one's",
			auditRow{Type: auditCourseEdit, Message: "Brauereiwesen:'brau' add: Anja (1)", CreatedAt: t0.AddDate(0, -3, 0)},
			auditLink{courseID: 6}, true,
		},
		{
			"a course edit of a since deleted course",
			auditRow{Type: auditCourseEdit, Message: "Analysis 1:'ana1' settings: x", CreatedAt: t0.AddDate(0, -8, 0)},
			auditLink{courseID: 3}, true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := r.resolve(tt.row)
			assert.Equal(t, tt.ok, ok)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestMigrate202610250LinksOnlyWhatItCanAndIsRepeatable(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:audit_backfill?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	for _, ddl := range []string{
		`CREATE TABLE courses (id integer primary key, name text, slug text, year integer, teaching_term text, created_at datetime, deleted_at datetime)`,
		`CREATE TABLE streams (id integer primary key, course_id integer, created_at datetime)`,
		`CREATE TABLE audits (id integer primary key, type integer, message text, created_at datetime, course_id integer, stream_id integer)`,
	} {
		require.NoError(t, db.Exec(ddl).Error)
	}
	require.NoError(t, db.Exec(`INSERT INTO courses VALUES (4,'Analysis 1','ana1',2025,'W',?,NULL)`, t0).Error)
	// A deleted course, which the entry about its deletion is all about.
	require.NoError(t, db.Exec(`INSERT INTO courses VALUES (3,'Analysis 1','ana1',2024,'W',?,?)`, t0.AddDate(-1, 0, 0), t0.AddDate(0, -6, 0)).Error)
	require.NoError(t, db.Exec(`INSERT INTO streams VALUES (100,4,?)`, t0).Error)
	require.NoError(t, db.Exec(`INSERT INTO audits (id,type,message,created_at) VALUES
		(1,8,'100: (Visibility: true)',?),
		(2,8,'999: ended (discard: false)',?),
		(3,11,'x (1) signed in as y (2)',?),
		(5,6,'''Analysis 1'' (2024, W)[3]. Token: abc',?)`, t0, t0, t0, t0).Error)
	// Already linked, to a course the message does not name: must be left as it is.
	require.NoError(t, db.Exec(`INSERT INTO audits (id,type,message,created_at,course_id) VALUES (4,8,'100: x',?,77)`, t0).Error)

	migrate := Migrate202610250().Migrate
	require.NoError(t, migrate(db))
	require.NoError(t, migrate(db))

	type row struct {
		ID       uint
		CourseID *uint
		StreamID *uint
	}
	var rows []row
	require.NoError(t, db.Table("audits").Order("id").Find(&rows).Error)
	require.Len(t, rows, 5)
	require.NotNil(t, rows[0].CourseID)
	assert.Equal(t, uint(4), *rows[0].CourseID)
	assert.Equal(t, uint(100), *rows[0].StreamID)
	assert.Nil(t, rows[1].CourseID, "an unknown stream stays server-wide")
	assert.Nil(t, rows[2].CourseID, "an impersonation is not about a course")
	assert.Equal(t, uint(77), *rows[3].CourseID, "a linked row is not rewritten")
	require.NotNil(t, rows[4].CourseID, "the deletion of a soft-deleted course is linked to it")
	assert.Equal(t, uint(3), *rows[4].CourseID)
}

func TestMigrate202610250SkipsAFreshDatabase(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, Migrate202610250().Migrate(db))
}

// More entries than one batch, with PrepareStmt as the server runs it: every batch must
// be linked. sqlite cannot reproduce MariaDB's max_prepared_stmt_count, so this guards
// the batching logic, not the limit itself (see auditBackfillBatch).
func TestMigrate202610250LinksAcrossBatches(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:audit_backfill_batches?mode=memory&cache=shared"), &gorm.Config{PrepareStmt: true})
	require.NoError(t, err)
	for _, ddl := range []string{
		`CREATE TABLE courses (id integer primary key, name text, slug text, year integer, teaching_term text, created_at datetime, deleted_at datetime)`,
		`CREATE TABLE streams (id integer primary key, course_id integer, created_at datetime)`,
		`CREATE TABLE audits (id integer primary key, type integer, message text, created_at datetime, course_id integer, stream_id integer)`,
	} {
		require.NoError(t, db.Exec(ddl).Error)
	}
	require.NoError(t, db.Exec(`INSERT INTO courses VALUES (4,'Analysis 1','ana1',2025,'W',?,NULL)`, t0).Error)
	require.NoError(t, db.Exec(`INSERT INTO streams VALUES (100,4,?)`, t0).Error)
	n := 2*auditBackfillBatch + 37
	for i := 1; i <= n; i++ {
		require.NoError(t, db.Exec(`INSERT INTO audits (id,type,message,created_at) VALUES (?,8,'100: x',?)`, i, t0).Error)
	}

	require.NoError(t, Migrate202610250().Migrate(db))

	var unlinked int64
	require.NoError(t, db.Table("audits").Where("course_id IS NULL").Count(&unlinked).Error)
	assert.Zero(t, unlinked)
}
