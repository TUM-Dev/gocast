package apiv2

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"google.golang.org/protobuf/types/known/emptypb"

	e "github.com/TUM-Dev/gocast/apiv2/errors"
	protobuf "github.com/TUM-Dev/gocast/apiv2/protobuf/server"
	"github.com/TUM-Dev/gocast/model"
)

var (
	errUnitNeedsTitle = errors.New("a unit needs a title")
	errUnitRange      = errors.New("a unit must end after it starts")
)

// ListLectureUnits is the units page's data, replacing the server-rendered page that
// embedded the units and the recording's playlist. The course-admin policy has
// checked the course; courseLecture checks the lecture is that course's.
func (a *API) ListLectureUnits(ctx context.Context, req *protobuf.ListLectureUnitsRequest) (*protobuf.ListLectureUnitsResponse, error) {
	stream, err := a.courseLecture(ctx, req.GetCourseId(), req.GetStreamId())
	if err != nil {
		return nil, err
	}

	user, _ := a.getCurrent(ctx)
	if err := signPlaylists(&stream, user, false); err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}
	// The old page offered the first recording it had, in this order.
	playlist := stream.PlaylistUrl
	if playlist == "" {
		playlist = stream.PlaylistUrlPRES
	}
	if playlist == "" {
		playlist = stream.PlaylistUrlCAM
	}

	var duration uint32
	if stream.Duration.Valid && stream.Duration.Int32 > 0 {
		duration = uint32(stream.Duration.Int32)
	}

	units := make([]*protobuf.LectureUnit, 0, len(stream.Units))
	for i := range stream.Units {
		units = append(units, lectureUnit(&stream.Units[i]))
	}
	return &protobuf.ListLectureUnitsResponse{
		LectureName:     stream.Name,
		PlaylistUrl:     playlist,
		DurationSeconds: duration,
		Units:           units,
	}, nil
}

func lectureUnit(u *model.StreamUnit) *protobuf.LectureUnit {
	return &protobuf.LectureUnit{
		Id:          uint32(u.ID),
		Title:       u.UnitName,
		Description: u.UnitDescription,
		FromMs:      uint32(u.UnitStart),
		ToMs:        uint32(u.UnitEnd),
	}
}

// CreateLectureUnit replaces v1's addUnit. Unlike v1, which accepted anything, it
// refuses a unit without a title and a range that is empty or runs backwards -- the
// uint subtraction in StreamUnit.GetUnitDurationMS would wrap around on one.
func (a *API) CreateLectureUnit(ctx context.Context, req *protobuf.CreateLectureUnitRequest) (*protobuf.LectureUnit, error) {
	stream, err := a.courseLecture(ctx, req.GetCourseId(), req.GetStreamId())
	if err != nil {
		return nil, err
	}

	title := strings.TrimSpace(req.GetTitle())
	if title == "" {
		return nil, e.WithStatus(http.StatusBadRequest, errUnitNeedsTitle)
	}
	if req.GetToMs() <= req.GetFromMs() {
		return nil, e.WithStatus(http.StatusBadRequest, errUnitRange)
	}

	unit := model.StreamUnit{
		UnitName:        title,
		UnitDescription: req.GetDescription(),
		UnitStart:       uint(req.GetFromMs()),
		UnitEnd:         uint(req.GetToMs()),
		StreamID:        stream.ID,
	}
	if err := a.dao.StreamsDao.CreateUnit(&unit); err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}
	return lectureUnit(&unit), nil
}

// DeleteLectureUnit replaces v1's deleteUnit. A unit of another lecture answers as a
// missing one, so a course's administrators cannot remove a foreign course's.
func (a *API) DeleteLectureUnit(ctx context.Context, req *protobuf.DeleteLectureUnitRequest) (*emptypb.Empty, error) {
	stream, err := a.courseLecture(ctx, req.GetCourseId(), req.GetStreamId())
	if err != nil {
		return nil, err
	}

	unit, err := a.dao.StreamsDao.GetUnitByID(strconv.FormatUint(uint64(req.GetUnitId()), 10))
	if err != nil {
		return nil, e.FromGorm(err, "can't find unit")
	}
	if unit.StreamID != stream.ID {
		return nil, e.WithStatus(http.StatusNotFound, errors.New("no such unit"))
	}
	a.dao.StreamsDao.DeleteUnit(unit.ID)
	return &emptypb.Empty{}, nil
}
