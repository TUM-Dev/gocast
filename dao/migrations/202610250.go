package migrations

import (
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

// The audit types the backfill reads, as stored. Spelled out rather than taken from
// model.AuditType: a migration describes the data as it was, and must not change
// meaning if the model's constants are ever reordered.
const (
	auditCourseCreate = 4
	auditCourseEdit   = 5
	auditCourseDelete = 6
	auditStreamCreate = 7
	auditStreamEdit   = 8
	auditStreamDelete = 9
)

// auditCreationWindow is how close an audit entry and the row it created are written:
// the entry follows the insert directly. Wide enough for a slow request, narrow enough
// that two courses of one name, created a semester apart, never both fit.
const auditCreationWindow = 10 * time.Second

// Migrate202610250 links audit entries that predate audits.course_id and stream_id to
// their course, so a course's own log shows its history and not only what happens from
// here on.
//
// The old entries carry no ids, only text, so each is resolved from what the message
// does state, and left alone unless that names exactly one course. A wrong link would
// show a course someone else's history, which is worse than a gap:
//
//   - stream edits and deletions name the stream's id, course deletions the course's;
//   - course and stream creations are matched by name (or slug, year and term) among
//     the courses and streams created within seconds of the entry, since a course's
//     name is repeated every semester;
//   - course edits and opt-ins name only the course's name and slug, and are resolved
//     among the courses that existed at the time.
//
// Entries that stay ambiguous are server-wide as before. Rows already linked are not
// touched, so rerunning is harmless.
func Migrate202610250() *gormigrate.Migration {
	return &gormigrate.Migration{
		ID: "202610250",
		Migrate: func(tx *gorm.DB) error {
			m := tx.Migrator()
			if !m.HasTable("audits") || !m.HasColumn("audits", "course_id") ||
				!m.HasTable("courses") || !m.HasTable("streams") {
				return nil
			}

			var audits []auditRow
			if err := tx.Table("audits").
				Select("id, type, message, created_at").
				Where("course_id IS NULL AND type IN ?", []int{
					auditCourseCreate, auditCourseEdit, auditCourseDelete, auditStreamCreate, auditStreamEdit, auditStreamDelete,
				}).
				Find(&audits).Error; err != nil {
				return err
			}
			if len(audits) == 0 {
				return nil
			}

			// Soft-deleted rows included: they keep their ids, and a deleted course's
			// history is what a deleted-course audit is about. Unscoped, because gorm
			// otherwise adds "deleted_at IS NULL" for the DeletedAt field below -- which
			// would drop almost every course-deleted entry, as those courses are deleted.
			var courses []auditCourse
			if err := tx.Table("courses").Unscoped().Select("id, name, slug, year, teaching_term, created_at, deleted_at").Find(&courses).Error; err != nil {
				return err
			}
			var streams []auditStream
			if err := tx.Table("streams").Unscoped().Select("id, course_id, created_at").Find(&streams).Error; err != nil {
				return err
			}

			resolver := newAuditResolver(courses, streams)
			return tx.Transaction(func(tx *gorm.DB) error {
				for _, a := range audits {
					link, ok := resolver.resolve(a)
					if !ok {
						continue
					}
					update := map[string]any{"course_id": link.courseID}
					if link.streamID != 0 {
						update["stream_id"] = link.streamID
					}
					if err := tx.Table("audits").Where("id = ? AND course_id IS NULL", a.ID).Updates(update).Error; err != nil {
						return err
					}
				}
				return nil
			})
		},
		Rollback: func(tx *gorm.DB) error {
			// The links are derived data; nothing is lost by leaving them in place.
			return nil
		},
	}
}

type auditRow struct {
	ID        uint
	Type      int
	Message   string
	CreatedAt time.Time
}

type auditCourse struct {
	ID           uint
	Name         string
	Slug         string
	Year         int
	TeachingTerm string
	CreatedAt    time.Time
	DeletedAt    gorm.DeletedAt
}

type auditStream struct {
	ID        uint
	CourseID  uint
	CreatedAt time.Time
}

type auditLink struct {
	courseID uint
	streamID uint // 0 when the entry is about the course, or the stream is ambiguous
}

var (
	streamEditMessage   = regexp.MustCompile(`^(\d+):`)
	streamDeleteMessage = regexp.MustCompile(`\((\d+)( and series)?\)$`)
	courseDeleteMessage = regexp.MustCompile(`\)\[(\d+)\]`)
	streamCreateMessage = regexp.MustCompile(`^Stream for '(.*)' Created\. Time:`)
	courseCreateMessage = regexp.MustCompile(`^([^:]+):'.*' \((\d+), ([WS])\)`)
	optedInPrefix       = "opted in by token, "
)

type auditResolver struct {
	courses       map[uint]auditCourse
	byName        map[string][]auditCourse
	streams       map[uint]auditStream
	streamsByCour map[uint][]auditStream
	all           []auditCourse
}

func newAuditResolver(courses []auditCourse, streams []auditStream) *auditResolver {
	r := &auditResolver{
		courses:       make(map[uint]auditCourse, len(courses)),
		byName:        map[string][]auditCourse{},
		streams:       make(map[uint]auditStream, len(streams)),
		streamsByCour: map[uint][]auditStream{},
		all:           courses,
	}
	for _, c := range courses {
		r.courses[c.ID] = c
		r.byName[c.Name] = append(r.byName[c.Name], c)
	}
	for _, s := range streams {
		r.streams[s.ID] = s
		r.streamsByCour[s.CourseID] = append(r.streamsByCour[s.CourseID], s)
	}
	return r
}

func within(a, b time.Time) bool {
	d := a.Sub(b)
	return d >= -auditCreationWindow && d <= auditCreationWindow
}

// aliveAt reports whether the course existed (and was not yet deleted) at t.
func (c auditCourse) aliveAt(t time.Time) bool {
	return !c.CreatedAt.After(t) && (!c.DeletedAt.Valid || !c.DeletedAt.Time.Before(t))
}

// only returns the one course of the list, if it is exactly one.
func only(courses []auditCourse) (auditCourse, bool) {
	if len(courses) != 1 {
		return auditCourse{}, false
	}
	return courses[0], true
}

func (r *auditResolver) resolve(a auditRow) (auditLink, bool) {
	switch a.Type {
	case auditStreamEdit:
		if m := streamEditMessage.FindStringSubmatch(a.Message); m != nil {
			return r.viaStream(m[1])
		}
	case auditStreamDelete:
		if m := streamDeleteMessage.FindStringSubmatch(a.Message); m != nil {
			return r.viaStream(m[1])
		}
	case auditCourseDelete:
		if m := courseDeleteMessage.FindStringSubmatch(a.Message); m != nil {
			id, _ := strconv.ParseUint(m[1], 10, 64)
			if _, ok := r.courses[uint(id)]; ok {
				return auditLink{courseID: uint(id)}, true
			}
		}
	case auditStreamCreate:
		return r.viaStreamCreation(a)
	case auditCourseCreate:
		return r.viaCourseCreation(a)
	case auditCourseEdit:
		return r.viaNameAndSlug(a, a.Message)
	}
	return auditLink{}, false
}

func (r *auditResolver) viaStream(rawID string) (auditLink, bool) {
	id, err := strconv.ParseUint(rawID, 10, 64)
	if err != nil {
		return auditLink{}, false
	}
	s, ok := r.streams[uint(id)]
	if !ok {
		return auditLink{}, false
	}
	return auditLink{courseID: s.CourseID, streamID: s.ID}, true
}

// viaStreamCreation: "Stream for '<course name>' Created. Time: <lecture time>" -- the
// lecture time is its own and moves when the lecture does, so the match is on when the
// stream row was written instead.
func (r *auditResolver) viaStreamCreation(a auditRow) (auditLink, bool) {
	m := streamCreateMessage.FindStringSubmatch(a.Message)
	if m == nil {
		return auditLink{}, false
	}
	var matched []auditCourse
	var matchedStreams []auditStream
	for _, c := range r.byName[m[1]] {
		var hits []auditStream
		for _, s := range r.streamsByCour[c.ID] {
			if within(s.CreatedAt, a.CreatedAt) {
				hits = append(hits, s)
			}
		}
		if len(hits) > 0 {
			matched = append(matched, c)
			matchedStreams = hits
		}
	}
	c, ok := only(matched)
	if !ok {
		return auditLink{}, false
	}
	link := auditLink{courseID: c.ID}
	// A batch of lectures is created in one request, so the window usually holds
	// several streams; the stream is only named when it is the one.
	if len(matchedStreams) == 1 {
		link.streamID = matchedStreams[0].ID
	}
	return link, true
}

// viaCourseCreation: "<slug>:'<name>' (<year>, <term>)" for creations and copies, and
// "opted in by token, <name>:'<slug>'" for opt-ins.
func (r *auditResolver) viaCourseCreation(a auditRow) (auditLink, bool) {
	if strings.HasPrefix(a.Message, optedInPrefix) {
		return r.viaNameAndSlug(a, strings.TrimPrefix(a.Message, optedInPrefix))
	}
	m := courseCreateMessage.FindStringSubmatch(a.Message)
	if m == nil {
		return auditLink{}, false
	}
	year, _ := strconv.Atoi(m[2])
	var matched []auditCourse
	for _, c := range r.all {
		if c.Slug == m[1] && c.Year == year && c.TeachingTerm == m[3] && within(c.CreatedAt, a.CreatedAt) {
			matched = append(matched, c)
		}
	}
	c, ok := only(matched)
	if !ok {
		return auditLink{}, false
	}
	return auditLink{courseID: c.ID}, true
}

// viaNameAndSlug resolves "<name>:'<slug>'..." among the courses that existed when the
// entry was written. Names contain colons, so each course is tried rather than the
// message split.
func (r *auditResolver) viaNameAndSlug(a auditRow, text string) (auditLink, bool) {
	var matched []auditCourse
	for _, c := range r.all {
		if strings.HasPrefix(text, c.Name+":'"+c.Slug+"'") && c.aliveAt(a.CreatedAt) {
			matched = append(matched, c)
		}
	}
	c, ok := only(matched)
	if !ok {
		return auditLink{}, false
	}
	return auditLink{courseID: c.ID}, true
}
