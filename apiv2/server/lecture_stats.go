package apiv2

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"google.golang.org/protobuf/types/known/timestamppb"

	e "github.com/TUM-Dev/gocast/apiv2/errors"
	protobuf "github.com/TUM-Dev/gocast/apiv2/protobuf/server"
)

// lecturePartialHistoryUntilYear is the last year whose lectures can predate the
// viewing data. The old page used it; the course page's cutoff is a year later
// because a course from 2022 can still hold lectures from 2021.
const lecturePartialHistoryUntilYear = 2021

// GetLectureStats is the lecture statistics page's data, replacing v1's getStats with
// a `lecture` parameter. The course-admin policy has checked the course; this checks
// the stream is that course's. v1 did not, and several of the per-lecture queries
// filter on the stream alone, so any course's administrators could read any
// lecture's view counts by naming it.
func (a *API) GetLectureStats(ctx context.Context, req *protobuf.GetLectureStatsRequest) (*protobuf.LectureStatsResponse, error) {
	stream, err := a.dao.GetStreamByID(ctx, fmt.Sprintf("%d", req.GetStreamId()))
	if err != nil {
		return nil, e.FromGorm(err, "can't find stream")
	}

	// Same answer as a missing stream, as authorizeCourseAdmin does for courses. Also
	// keeps course 0, which the course-wide queries read as every course, unreachable:
	// no stream belongs to it.
	if stream.CourseID != uint(req.GetCourseId()) || stream.CourseID == serverStatsCourseID {
		return nil, e.WithStatus(http.StatusNotFound, errors.New("no such stream"))
	}

	course, err := a.dao.GetCourseById(ctx, stream.CourseID)
	if err != nil {
		return nil, e.FromGorm(err, "can't find course")
	}

	numStudents, err := a.dao.StatisticsDao.GetCourseNumStudents(stream.CourseID)
	if err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}
	vodViews, err := a.dao.StatisticsDao.GetLectureNumVodViews(stream.ID)
	if err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}
	maxLiveViews, err := a.dao.StatisticsDao.GetLectureNumLiveViews(stream.ID)
	if err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}
	liveViewers, err := a.dao.StatisticsDao.GetLectureStats(stream.CourseID, stream.ID)
	if err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}
	weekdays, err := a.dao.StatisticsDao.GetLectureStatsWeekdays(stream.CourseID, stream.ID)
	if err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}
	allDays, err := a.dao.StatisticsDao.GetLectureNumVodViewsPerDay(stream.ID)
	if err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}

	// Labels match the chartJs datasets v1 answered with.
	return &protobuf.LectureStatsResponse{
		CourseName:     course.Name,
		LectureName:    stream.Name,
		Start:          timestamppb.New(stream.Start),
		End:            timestamppb.New(stream.End),
		NumStudents:    numStudents,
		VodViews:       int32(vodViews),
		MaxLiveViews:   int32(maxLiveViews),
		LiveViewers:    serverStatsSeries("View Count", liveViewers),
		Weekdays:       serverStatsSeries("Sum(viewers)", weekdays),
		AllDays:        serverStatsSeries("views", allDays),
		PartialHistory: stream.Start.Year() <= lecturePartialHistoryUntilYear,
	}, nil
}
