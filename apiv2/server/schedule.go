package apiv2

import (
	"context"
	"errors"
	"net/http"
	"sort"
	"time"

	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	e "github.com/TUM-Dev/gocast/apiv2/errors"
	protobuf "github.com/TUM-Dev/gocast/apiv2/protobuf/server"
	"github.com/TUM-Dev/gocast/model"
)

// maxScheduleWindow bounds one getSchedule call. The page asks for the week or day it
// shows; this only stops a single request from reading every lecture there is.
const maxScheduleWindow = 100 * 24 * time.Hour

// GetSchedule replaces v1's schedule.ics for the administration schedule. v1 read a
// fixed window from a month ago to three months ahead, so the calendar showed nothing
// outside it; this reads whatever the calendar is showing. Gated on the lecture
// permission by its policy.
func (a *API) GetSchedule(ctx context.Context, req *protobuf.GetScheduleRequest) (*protobuf.GetScheduleResponse, error) {
	user, err := a.getCurrent(ctx)
	if err != nil {
		return nil, e.WithStatus(http.StatusUnauthorized, err)
	}

	if req.GetFrom() == nil || req.GetTo() == nil {
		return nil, e.WithStatus(http.StatusBadRequest, errors.New("from and to are required"))
	}
	from, to := req.GetFrom().AsTime(), req.GetTo().AsTime()
	if !to.After(from) {
		return nil, e.WithStatus(http.StatusBadRequest, errors.New("to must be after from"))
	}
	if to.Sub(from) > maxScheduleWindow {
		return nil, e.WithStatus(http.StatusBadRequest, errors.New("the window may be at most 100 days"))
	}

	// 0 is GetSchedule's "every course", as it was the ical query's.
	userID := user.ID
	if user.Can(model.PermViewAllCourses) {
		userID = 0
	}

	halls := make([]uint, 0, len(req.GetLectureHallIds()))
	for _, id := range req.GetLectureHallIds() {
		halls = append(halls, uint(id))
	}

	entries, err := a.dao.LectureHallsDao.GetSchedule(userID, from, to, halls, req.GetAllLectureHalls())
	if err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}

	lectures := make([]*protobuf.ScheduledLecture, 0, len(entries))
	for _, entry := range entries {
		lectures = append(lectures, &protobuf.ScheduledLecture{
			StreamId:        uint32(entry.StreamID),
			CourseId:        uint32(entry.CourseID),
			CourseName:      entry.CourseName,
			Name:            entry.Name,
			Description:     entry.Description,
			Start:           timestamppb.New(entry.Start),
			End:             timestamppb.New(entry.End),
			LectureHallId:   uint32(entry.LectureHallID),
			LectureHallName: entry.LectureHallName,
		})
	}
	return &protobuf.GetScheduleResponse{Lectures: lectures}, nil
}

// ListScheduleLectureHalls names the halls the schedule can be filtered by. The
// template rendered the same list into the page for every lecturer; v2's
// listLectureHallsAdmin carries addresses and needs server.administer.
func (a *API) ListScheduleLectureHalls(ctx context.Context, _ *emptypb.Empty) (*protobuf.ListScheduleLectureHallsResponse, error) {
	halls := a.dao.LectureHallsDao.GetAllLectureHalls()
	sort.Slice(halls, func(i, j int) bool { return halls[i].Name < halls[j].Name })

	res := make([]*protobuf.ScheduleLectureHall, 0, len(halls))
	for _, hall := range halls {
		res = append(res, &protobuf.ScheduleLectureHall{Id: uint32(hall.ID), Name: hall.Name})
	}
	return &protobuf.ListScheduleLectureHallsResponse{LectureHalls: res}, nil
}
