package apiv2

import (
	"context"
	"errors"
	"net/http"

	"google.golang.org/genproto/googleapis/api/httpbody"

	e "github.com/TUM-Dev/gocast/apiv2/errors"
	protobuf "github.com/TUM-Dev/gocast/apiv2/protobuf/server"
)

// partialHistoryUntilYear is the last year whose courses can predate the viewing data,
// which is only collected from 28 June 2021. The old page used the same cutoff.
const partialHistoryUntilYear = 2022

// errNoSuchCourse matches what authorizeCourseAdmin answers for a missing course.
var errNoSuchCourse = errors.New("no such course")

// GetCourseStats is the course statistics page's data. The course-admin policy has
// checked the caller administers the course, and refused course 0, which the
// statistics queries would read as every course.
func (a *API) GetCourseStats(ctx context.Context, req *protobuf.GetCourseStatsRequest) (*protobuf.CourseStatsResponse, error) {
	course, err := a.dao.GetCourseById(ctx, uint(req.GetCourseId()))
	if err != nil {
		return nil, e.FromGorm(err, "can't find course")
	}

	// GetCourseById reports a missing row as a zero-value course, and course ID 0
	// is how the statistics queries spell "every course" -- so this, not the policy
	// alone, is what keeps a course's lecturers from the server-wide numbers.
	if course.ID == serverStatsCourseID {
		return nil, e.WithStatus(http.StatusNotFound, errNoSuchCourse)
	}

	stats, err := a.collectUsageStats(course.ID)
	if err != nil {
		return nil, err
	}

	return &protobuf.CourseStatsResponse{
		CourseName:     course.Name,
		NumStudents:    stats.numStudents,
		VodViews:       int32(stats.vodViews),
		LiveViews:      int32(stats.liveViews),
		NumLectures:    int32(course.NumStreams()),
		ActivityLive:   serverStatsSeries("Live", stats.activityLive),
		ActivityVod:    serverStatsSeries("VoD", stats.activityVod),
		Hourly:         serverStatsSeries("Sum(viewers)", stats.hourly),
		Weekdays:       serverStatsSeries("Sum(viewers)", stats.weekdays),
		AllDays:        serverStatsSeries("views", stats.allDays),
		PartialHistory: course.Year <= partialHistoryUntilYear,
	}, nil
}

// ExportCourseStats is GetCourseStats as a downloadable file, replacing v1's
// exportStats for a course.
func (a *API) ExportCourseStats(ctx context.Context, req *protobuf.ExportCourseStatsRequest) (*httpbody.HttpBody, error) {
	// Defensive: the policy already refuses course 0, and this must never fall
	// through to the server-wide numbers if that ever changes.
	if req.GetCourseId() == serverStatsCourseID {
		return nil, e.WithStatus(http.StatusNotFound, errNoSuchCourse)
	}

	return a.exportUsageStats(uint(req.GetCourseId()), req.GetFormat())
}
