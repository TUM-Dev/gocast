package apiv2

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	uuid "github.com/satori/go.uuid"
	"google.golang.org/protobuf/types/known/timestamppb"
	"gorm.io/gorm"

	e "github.com/TUM-Dev/gocast/apiv2/errors"
	protobuf "github.com/TUM-Dev/gocast/apiv2/protobuf/server"
	"github.com/TUM-Dev/gocast/model"
	"github.com/TUM-Dev/gocast/tools/safepath"
)

// maxLecturesPerRequest bounds a date series. v1 had no bound; a daily series over a
// whole semester stays well below it.
const maxLecturesPerRequest = 200

// adHocDelay is how far ahead an ad-hoc livestream starts, as in v1: the stream cron
// runs every minute and only picks up lectures that are due, so a start of "now"
// could be missed.
const adHocDelay = 2 * time.Minute

// placeholderDuration is how long a VOD upload or premiere created without a
// duration lasts in the schedule. v1's createVOD, which the old page actually used
// for uploads, gave an hour; the recording's own length is what the player shows.
const placeholderDuration = time.Hour

var errHallForRecording = errors.New("a VOD upload or premiere takes no lecture hall")

// CreateLectures replaces v1's createLecture and, for uploads, its createVOD. The
// course-admin policy has checked the caller administers the course.
//
// Differences from v1, each deliberate:
//   - A VOD upload is what createVOD made, the call the old page used: a lecture
//     waiting for its recording. v1's createLecture `vodup` branch handed a file that
//     did not exist yet to the LRZ upload script and so always failed.
//   - Only a livestream gets a stream key; ingest finds a lecture by it, and nothing
//     is ever streamed into an upload or a premiere.
//   - Each premiere of a series gets its own file, named by its own start. v1 named
//     every one after the first start, so a series shared one file.
//   - The lectures are created in one insert, all or none, rather than by saving the
//     course with its streams appended.
//   - v1 also kicked the workers after an ad-hoc lecture, but called NotifyWorkers
//     without running the function it returns, so the minutely cron always did it;
//     it still does.
//
// Creating a lecture in a hall is left to any course administrator, as in v1, where
// the create form offered the hall to lecturers too; only changing an existing
// lecture's hall needs a server administrator (mayChangeLectureHall).
func (a *API) CreateLectures(ctx context.Context, req *protobuf.CreateLecturesRequest) (*protobuf.CreateLecturesResponse, error) {
	if req.GetCourseId() == 0 {
		return nil, e.WithStatus(http.StatusNotFound, errors.New("no such course"))
	}
	title := strings.TrimSpace(req.GetTitle())
	if title == "" {
		return nil, e.WithStatus(http.StatusBadRequest, errors.New("title is required"))
	}

	kind := req.GetKind()
	livestream := kind == protobuf.LectureCreationKind_LECTURE_CREATION_KIND_LIVESTREAM
	switch kind {
	case protobuf.LectureCreationKind_LECTURE_CREATION_KIND_LIVESTREAM,
		protobuf.LectureCreationKind_LECTURE_CREATION_KIND_VOD_UPLOAD,
		protobuf.LectureCreationKind_LECTURE_CREATION_KIND_PREMIERE:
	default:
		return nil, e.WithStatus(http.StatusBadRequest, fmt.Errorf("unknown kind %d", kind))
	}
	if !livestream && req.GetLectureHallId() != 0 {
		return nil, e.WithStatus(http.StatusBadRequest, errHallForRecording)
	}
	if req.GetAdHoc() && !livestream {
		return nil, e.WithStatus(http.StatusBadRequest, errors.New("only a livestream can start ad hoc"))
	}

	duration := time.Duration(req.GetDurationMinutes()) * time.Minute
	if duration == 0 {
		if livestream {
			return nil, e.WithStatus(http.StatusBadRequest, errors.New("duration_minutes is required for a livestream"))
		}
		duration = placeholderDuration
	}

	starts, err := lectureStarts(req)
	if err != nil {
		return nil, err
	}

	course, err := a.dao.GetCourseById(ctx, uint(req.GetCourseId()))
	if err != nil {
		return nil, e.FromGorm(err, "can't find course")
	}
	if course.ID == 0 {
		return nil, e.WithStatus(http.StatusNotFound, errors.New("no such course"))
	}

	hallNames := map[uint]string{}
	if hallID := req.GetLectureHallId(); hallID != 0 {
		// The request's mistake rather than a missing resource, as in
		// updateLecturesLectureHall: the course is what the path names.
		hall, err := a.dao.LectureHallsDao.GetLectureHallByID(uint(hallID))
		if errors.Is(err, gorm.ErrRecordNotFound) || (err == nil && hall.ID == 0) {
			return nil, e.WithStatus(http.StatusBadRequest, errors.New("no such lecture hall"))
		}
		if err != nil {
			return nil, e.WithStatus(http.StatusInternalServerError, err)
		}
		hallNames[hall.ID] = hall.Name
	}

	var premiereDir string
	if kind == protobuf.LectureCreationKind_LECTURE_CREATION_KIND_PREMIERE {
		if premiereDir, err = a.premiereDir(course); err != nil {
			return nil, err
		}
	}

	seriesIdentifier := ""
	if len(starts) > 1 {
		seriesIdentifier = uuid.NewV4().String()
	}

	lectures := make([]model.Stream, 0, len(starts))
	for _, start := range starts {
		lecture := model.Stream{
			Name:             title,
			CourseID:         course.ID,
			LectureHallID:    uint(req.GetLectureHallId()),
			Start:            start,
			End:              start.Add(duration),
			ChatEnabled:      req.GetChatEnabled(),
			SeriesIdentifier: seriesIdentifier,
			Premiere:         kind == protobuf.LectureCreationKind_LECTURE_CREATION_KIND_PREMIERE,
		}
		if livestream {
			lecture.StreamKey = strings.ReplaceAll(uuid.NewV4().String(), "-", "")
		}
		if lecture.Premiere {
			// Where v1 put it, and where the worker streams it from at start.
			lecture.Files = []model.File{{Path: fmt.Sprintf("%s/%s_%s.mp4",
				premiereDir, safepath.Component(course.Slug), start.Format("2006-01-02_15-04"))}}
		}
		lectures = append(lectures, lecture)
	}

	if err := a.dao.StreamsDao.CreateStreams(lectures); err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}

	resp := &protobuf.CreateLecturesResponse{Lectures: make([]*protobuf.CourseLectureAdmin, 0, len(lectures))}
	for i := range lectures {
		a.audit(ctx, model.AuditStreamCreate, fmt.Sprintf("Stream for '%s' Created. Time: %s",
			course.Name, lectures[i].Start.Format("2006 02 Jan, 15:04")))
		resp.Lectures = append(resp.Lectures, courseLectureAdmin(&lectures[i], hallNames))
	}
	return resp, nil
}

// lectureStarts is when each lecture of the request starts: start, then every date of
// the series. An ad-hoc lecture starts shortly from now and has no series.
func lectureStarts(req *protobuf.CreateLecturesRequest) ([]time.Time, error) {
	if req.GetAdHoc() {
		if len(req.GetDateSeries()) > 0 {
			return nil, e.WithStatus(http.StatusBadRequest, errors.New("an ad-hoc lecture has no date series"))
		}
		return []time.Time{time.Now().Add(adHocDelay)}, nil
	}

	if req.GetStart() == nil {
		return nil, e.WithStatus(http.StatusBadRequest, errors.New("start is required"))
	}
	if len(req.GetDateSeries())+1 > maxLecturesPerRequest {
		return nil, e.WithStatus(http.StatusBadRequest,
			fmt.Errorf("at most %d lectures can be created at once", maxLecturesPerRequest))
	}

	starts := make([]time.Time, 0, len(req.GetDateSeries())+1)
	seen := map[int64]bool{}
	for _, ts := range append([]*timestamppb.Timestamp{req.GetStart()}, req.GetDateSeries()...) {
		if ts == nil || !ts.IsValid() {
			return nil, e.WithStatus(http.StatusBadRequest, errors.New("invalid date"))
		}
		start := ts.AsTime()
		// v1 did not look, and the old page appended its own dates after start, so a
		// client repeating start would have doubled the lecture.
		if seen[start.UnixNano()] {
			return nil, e.WithStatus(http.StatusBadRequest, fmt.Errorf("the date %s is given twice", start.Format(time.RFC3339)))
		}
		seen[start.UnixNano()] = true
		starts = append(starts, start)
	}
	return starts, nil
}

// premiereDir creates the directory a premiere's file is expected in, as v1 did so
// an operator could put it there: <mass storage>/<year>/<term>/<course slug>. Through
// safepath, since the slug is the course's to choose.
func (a *API) premiereDir(course model.Course) (string, error) {
	if a.massStorage == "" {
		return "", e.WithStatus(http.StatusServiceUnavailable, errors.New("no directory configured for premieres"))
	}
	dir, err := safepath.JoinInRoot(a.massStorage, fmt.Sprintf("%d", course.Year), course.TeachingTerm, course.Slug)
	if err != nil {
		return "", e.WithStatus(http.StatusInternalServerError, err)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", e.WithStatus(http.StatusInternalServerError, fmt.Errorf("can't create folder for the premiere: %w", err))
	}
	return dir, nil
}
