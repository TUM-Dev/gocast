package apiv2

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"google.golang.org/protobuf/types/known/emptypb"

	e "github.com/TUM-Dev/gocast/apiv2/errors"
	protobuf "github.com/TUM-Dev/gocast/apiv2/protobuf/server"
	"github.com/TUM-Dev/gocast/model"
)

var errCutEndBeforeStart = errors.New("the cut must end after it starts")

// GetLectureCut is the cut page's data, replacing the template's use of the stream
// from the request context. The course-admin policy has checked the course; this
// checks the stream is that course's.
func (a *API) GetLectureCut(ctx context.Context, req *protobuf.GetLectureCutRequest) (*protobuf.LectureCutResponse, error) {
	stream, err := a.courseLecture(ctx, req.GetCourseId(), req.GetStreamId())
	if err != nil {
		return nil, err
	}
	course, err := a.dao.GetCourseById(ctx, stream.CourseID)
	if err != nil {
		return nil, e.FromGorm(err, "can't find course")
	}

	user, _ := a.getCurrent(ctx)
	if err := signPlaylists(&stream, user, false); err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}
	// The order the old page chose its reference video in.
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

	return &protobuf.LectureCutResponse{
		CourseName:      course.Name,
		LectureName:     stream.Name,
		PlaylistUrl:     playlist,
		StartOffsetMs:   uint32(stream.StartOffset),
		EndOffsetMs:     uint32(stream.EndOffset),
		DurationSeconds: duration,
	}, nil
}

// SetLectureCut replaces v1's submitCut. The offsets are stored as sent, as v1 did,
// except that both being zero is accepted and removes the cut: v1 saved through
// SaveStream, which skips zero values, so a cut could never be undone from the page
// that said it was reversible.
func (a *API) SetLectureCut(ctx context.Context, req *protobuf.SetLectureCutRequest) (*emptypb.Empty, error) {
	stream, err := a.courseLecture(ctx, req.GetCourseId(), req.GetStreamId())
	if err != nil {
		return nil, err
	}

	start, end := req.GetStartOffsetMs(), req.GetEndOffsetMs()
	if (start != 0 || end != 0) && end <= start {
		return nil, e.WithStatus(http.StatusBadRequest, errCutEndBeforeStart)
	}

	if err := a.dao.StreamsDao.SetCut(stream.ID, uint(start), uint(end)); err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}
	a.audit(ctx, model.AuditStreamEdit, fmt.Sprintf("%d: (Cut: %d-%d ms)", stream.ID, start, end))
	return &emptypb.Empty{}, nil
}
