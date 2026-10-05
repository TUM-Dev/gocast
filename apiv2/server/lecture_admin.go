package apiv2

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	uuid "github.com/satori/go.uuid"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
	"gorm.io/gorm"

	e "github.com/TUM-Dev/gocast/apiv2/errors"
	protobuf "github.com/TUM-Dev/gocast/apiv2/protobuf/server"
	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/model"
	"github.com/TUM-Dev/gocast/pkg/realtimehub"
)

var (
	errNoSuchStream     = errors.New("no such stream")
	errNotInSeries      = errors.New("the lecture is not in a lecture series")
	errStartEndTogether = errors.New("start and end must be given together")
	errEndBeforeStart   = errors.New("end must be after start")
)

// courseLecture loads a lecture and checks it is the course's. The course-admin policy
// has checked the caller administers the course, and nothing more: v1's InitStream
// took the stream from the URL without comparing it to the course, so any course's
// administrators could act on any lecture by naming it. Another course's lecture
// answers as a missing one, as authorizeCourseAdmin does for courses.
func (a *API) courseLecture(ctx context.Context, courseID, streamID uint32) (model.Stream, error) {
	stream, err := a.dao.GetStreamByID(ctx, fmt.Sprintf("%d", streamID))
	if err != nil {
		return stream, e.FromGorm(err, "can't find stream")
	}
	if stream.CourseID != uint(courseID) {
		return model.Stream{}, e.WithStatus(http.StatusNotFound, errNoSuchStream)
	}
	return stream, nil
}

// lectureTimes reads an optional start/end pair: both or neither, end after start.
// ok reports whether there was a pair.
func lectureTimes(start, end *timestamppb.Timestamp) (s, en time.Time, ok bool, err error) {
	if start == nil && end == nil {
		return time.Time{}, time.Time{}, false, nil
	}
	if start == nil || end == nil {
		return time.Time{}, time.Time{}, false, e.WithStatus(http.StatusBadRequest, errStartEndTogether)
	}
	s, en = start.AsTime(), end.AsTime()
	if !en.After(s) {
		return time.Time{}, time.Time{}, false, e.WithStatus(http.StatusBadRequest, errEndBeforeStart)
	}
	return s, en, true, nil
}

// checkLectureHall answers 404 for a hall that does not exist; 0 (none) always does.
func (a *API) checkLectureHall(id uint32) error {
	if id == 0 {
		return nil
	}
	hall, err := a.dao.LectureHallsDao.GetLectureHallByID(uint(id))
	if err != nil {
		return e.FromGorm(err, "can't find lecture hall")
	}
	if hall.ID == 0 {
		return e.WithStatus(http.StatusNotFound, errors.New("no such lecture hall"))
	}
	return nil
}

var errHallNeedsServerAdmin = errors.New("only server administrators may change a lecture's hall")

// mayChangeLectureHall refuses a caller who is not a server administrator. A hall is
// shared infrastructure, scheduled across every course, so a course's lecturers do
// not get to move their lectures into or out of one. v1's page hid the hall select
// from lecturers (`{{if eq $user.Role 1}}`) but its handler took the field from any
// course administrator; v2 enforces what the page meant.
func (a *API) mayChangeLectureHall(ctx context.Context) error {
	user, err := a.getCurrent(ctx)
	if err != nil {
		return e.WithStatus(http.StatusUnauthorized, err)
	}
	if !user.Can(model.PermAdministerServer) {
		return e.WithStatus(http.StatusForbidden, errHallNeedsServerAdmin)
	}
	return nil
}

// audit records an administrator's change. A failure is logged, not answered: the
// change itself has been made.
func (a *API) audit(ctx context.Context, typ model.AuditType, message string) {
	user, _ := a.getCurrent(ctx)
	if err := a.dao.AuditDao.Create(&model.Audit{User: user, Message: message, Type: typ}); err != nil {
		a.log.Error("can't write audit entry", "err", err, "message", message)
	}
}

// ListCourseLecturesAdmin is the lecture management list, replacing v1's
// GET /api/course/:courseID/lectures. Gated on administering the course.
//
// v1 also answered signed playlist and download URLs, a colour and hasStats; the
// colour is the client's to derive from the state flags, hasStats was always false
// (the stats were never loaded), and the URLs are the player's business, so only
// which versions exist is carried here.
func (a *API) ListCourseLecturesAdmin(ctx context.Context, req *protobuf.ListCourseLecturesAdminRequest) (*protobuf.ListCourseLecturesAdminResponse, error) {
	course, err := a.dao.GetCourseById(ctx, uint(req.GetCourseId()))
	if err != nil {
		return nil, e.FromGorm(err, "can't find course")
	}

	hallNames := map[uint]string{}
	for _, hall := range a.dao.LectureHallsDao.GetAllLectureHalls() {
		hallNames[hall.ID] = hall.Name
	}

	lectures := make([]*protobuf.CourseLectureAdmin, 0, len(course.Streams))
	for i := range course.Streams {
		lectures = append(lectures, courseLectureAdmin(&course.Streams[i], hallNames))
	}
	return &protobuf.ListCourseLecturesAdminResponse{Lectures: lectures}, nil
}

func courseLectureAdmin(s *model.Stream, hallNames map[uint]string) *protobuf.CourseLectureAdmin {
	var versions []string
	if s.PlaylistUrl != "" {
		versions = append(versions, string(model.COMB))
	}
	if s.PlaylistUrlPRES != "" {
		versions = append(versions, string(model.PRES))
	}
	if s.PlaylistUrlCAM != "" {
		versions = append(versions, string(model.CAM))
	}

	files := make([]*protobuf.LectureFileAdmin, 0, len(s.Files))
	for _, f := range s.Files {
		files = append(files, &protobuf.LectureFileAdmin{
			Id: uint32(f.ID), Type: uint32(f.Type), FriendlyName: f.GetFriendlyFileName(),
		})
	}

	progresses := make([]*protobuf.LectureTranscodingProgress, 0, len(s.TranscodingProgresses))
	for _, p := range s.TranscodingProgresses {
		progresses = append(progresses, &protobuf.LectureTranscodingProgress{
			Version: string(p.Version), Progress: int32(p.Progress),
		})
	}

	sections := make([]*protobuf.LectureVideoSectionAdmin, 0, len(s.VideoSections))
	for _, v := range s.VideoSections {
		sections = append(sections, &protobuf.LectureVideoSectionAdmin{
			Id: uint32(v.ID), Description: v.Description,
			StartHours: uint32(v.StartHours), StartMinutes: uint32(v.StartMinutes), StartSeconds: uint32(v.StartSeconds),
			FileId: uint32(v.FileID),
		})
	}

	var duration uint32
	if s.Duration.Valid && s.Duration.Int32 > 0 {
		duration = uint32(s.Duration.Int32)
	}

	return &protobuf.CourseLectureAdmin{
		Id:                     uint32(s.ID),
		CourseId:               uint32(s.CourseID),
		Name:                   s.Name,
		Description:            s.Description,
		Start:                  timestamppb.New(s.Start),
		End:                    timestamppb.New(s.End),
		LectureHallId:          uint32(s.LectureHallID),
		LectureHallName:        hallNames[s.LectureHallID],
		SeriesIdentifier:       s.SeriesIdentifier,
		StreamKey:              s.StreamKey,
		ChatEnabled:            s.ChatEnabled,
		Private:                s.Private,
		CustomThumbnailEnabled: s.CustomThumbnailEnabled,
		LiveNow:                s.LiveNow,
		Recording:              s.Recording,
		Past:                   s.IsPast(),
		Converting:             s.IsConverting(),
		Premiere:               s.Premiere,
		VodVersions:            versions,
		DurationSeconds:        duration,
		Files:                  files,
		TranscodingProgresses:  progresses,
		VideoSections:          sections,
	}
}

// UpdateLecture replaces v1's renameLecture, updateDescription, updateLectureTime,
// setLectureHall for one lecture, and the stream chat and visibility toggles. The
// course-admin policy has checked the course; courseLecture checks the lecture is
// that course's, which v1 did not.
//
// Everything is validated before anything is written, so a bad hall or time leaves
// the lecture as it was.
//
// Renames and description changes reach everyone watching over the realtime socket,
// as v1 pushed them over its own. Time changes do not: RealtimeEvent has no kind for
// them yet, so viewers see those when they next load the page.
func (a *API) UpdateLecture(ctx context.Context, req *protobuf.UpdateLectureRequest) (*emptypb.Empty, error) {
	stream, err := a.courseLecture(ctx, req.GetCourseId(), req.GetStreamId())
	if err != nil {
		return nil, err
	}

	start, end, hasTimes, err := lectureTimes(req.GetStart(), req.GetEnd())
	if err != nil {
		return nil, err
	}
	if req.LectureHallId != nil {
		if err := a.mayChangeLectureHall(ctx); err != nil {
			return nil, err
		}
		if err := a.checkLectureHall(req.GetLectureHallId()); err != nil {
			return nil, err
		}
	}

	if req.Name != nil {
		stream.Name = strings.TrimSpace(req.GetName())
	}
	if req.Description != nil {
		stream.Description = req.GetDescription()
	}
	if hasTimes {
		stream.Start, stream.End = start, end
	}
	if req.ChatEnabled != nil {
		stream.ChatEnabled = req.GetChatEnabled()
	}

	if req.LectureHallId != nil {
		// As v1's setLectureHall: only the assignment changes. The hall's sources are
		// read when the lecture is streamed, and the stream key is the lecture's own.
		ids := []uint{stream.ID}
		if req.GetLectureHallId() == 0 {
			err = a.dao.StreamsDao.UnsetLectureHall(ids)
		} else {
			err = a.dao.StreamsDao.SetLectureHall(ids, uint(req.GetLectureHallId()))
		}
		if err != nil {
			return nil, e.WithStatus(http.StatusInternalServerError, err)
		}
	}

	if req.Private != nil {
		if err := a.dao.StreamsDao.ToggleVisibility(stream.ID, req.GetPrivate()); err != nil {
			return nil, e.WithStatus(http.StatusInternalServerError, err)
		}
		// v1's visibility toggle audited; its other edits did not.
		a.audit(ctx, model.AuditStreamEdit, fmt.Sprintf("%d: (Visibility: %v)", stream.ID, req.GetPrivate()))
	}

	// Last, also because it is what clears the stream cache the hall update skips.
	if err := a.dao.StreamsDao.UpdateStream(stream); err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}

	if req.Name != nil {
		a.publish(ctx, stream.ID, realtimehub.TitleEvent(stream.ID, stream.Name))
	}
	if req.Description != nil {
		a.publish(ctx, stream.ID, realtimehub.DescriptionEvent(stream.ID, stream.GetDescriptionHTML()))
	}
	return &emptypb.Empty{}, nil
}

// UpdateLectureSeries replaces v1's updateLectureSeries, which copied a lecture's
// already-saved name and description to its series. This takes the values instead,
// and also sets the hall and chat, which the old page had to send per lecture.
//
// Narrowed to the course: v1 matched on the series identifier alone, which a copied
// lecture carries into its new course.
func (a *API) UpdateLectureSeries(ctx context.Context, req *protobuf.UpdateLectureSeriesRequest) (*emptypb.Empty, error) {
	stream, err := a.courseLecture(ctx, req.GetCourseId(), req.GetStreamId())
	if err != nil {
		return nil, err
	}
	if stream.SeriesIdentifier == "" {
		return nil, e.WithStatus(http.StatusBadRequest, errNotInSeries)
	}

	update := dao.LectureSeriesUpdate{ChatEnabled: req.ChatEnabled, Description: req.Description}
	if req.Name != nil {
		name := strings.TrimSpace(req.GetName())
		update.Name = &name
	}
	if req.LectureHallId != nil {
		if err := a.mayChangeLectureHall(ctx); err != nil {
			return nil, err
		}
		if err := a.checkLectureHall(req.GetLectureHallId()); err != nil {
			return nil, err
		}
		hall := uint(req.GetLectureHallId())
		update.LectureHallID = &hall
	}

	if err := a.dao.StreamsDao.UpdateCourseLectureSeries(stream.CourseID, stream.SeriesIdentifier, update); err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}
	return &emptypb.Empty{}, nil
}

// UpdateLectureSeriesTime replaces v1's updateLectureTime followed by
// updateLectureSeriesTime, the two calls the old page made in turn: the lecture takes
// the new times and the rest of its series their time of day and duration, each
// keeping its date. In one transaction rather than two requests, and narrowed to the
// course as UpdateLectureSeries is.
func (a *API) UpdateLectureSeriesTime(ctx context.Context, req *protobuf.UpdateLectureSeriesTimeRequest) (*emptypb.Empty, error) {
	stream, err := a.courseLecture(ctx, req.GetCourseId(), req.GetStreamId())
	if err != nil {
		return nil, err
	}

	start, end, ok, err := lectureTimes(req.GetStart(), req.GetEnd())
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, e.WithStatus(http.StatusBadRequest, errors.New("start and end are required"))
	}
	if stream.SeriesIdentifier == "" {
		return nil, e.WithStatus(http.StatusBadRequest, errNotInSeries)
	}

	if err := a.dao.StreamsDao.UpdateCourseLectureSeriesTime(stream.CourseID, stream.ID, stream.SeriesIdentifier, start, end); err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}
	return &emptypb.Empty{}, nil
}

// UpdateLecturesLectureHall replaces v1's setLectureHall for the lectures selected on
// the course page. Server administrators only, as for the hall in UpdateLecture.
//
// Nothing is written unless every lecture is the course's, and an unknown hall is
// the request's mistake rather than a missing resource: the lectures are what the
// path names.
func (a *API) UpdateLecturesLectureHall(ctx context.Context, req *protobuf.UpdateLecturesLectureHallRequest) (*emptypb.Empty, error) {
	if len(req.GetStreamIds()) == 0 {
		return nil, e.WithStatus(http.StatusBadRequest, errors.New("no lectures given"))
	}
	if err := a.mayChangeLectureHall(ctx); err != nil {
		return nil, err
	}

	ids := make([]uint, 0, len(req.GetStreamIds()))
	for _, id := range req.GetStreamIds() {
		stream, err := a.courseLecture(ctx, req.GetCourseId(), id)
		if err != nil {
			return nil, err
		}
		ids = append(ids, stream.ID)
	}

	var err error
	if req.GetLectureHallId() == 0 {
		err = a.dao.StreamsDao.UnsetLectureHall(ids)
	} else {
		hall, hallErr := a.dao.LectureHallsDao.GetLectureHallByID(uint(req.GetLectureHallId()))
		if errors.Is(hallErr, gorm.ErrRecordNotFound) || (hallErr == nil && hall.ID == 0) {
			return nil, e.WithStatus(http.StatusBadRequest, errors.New("no such lecture hall"))
		}
		if hallErr != nil {
			return nil, e.WithStatus(http.StatusInternalServerError, hallErr)
		}
		err = a.dao.StreamsDao.SetLectureHall(ids, hall.ID)
	}
	if err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}
	return &emptypb.Empty{}, nil
}

// DeleteLectures replaces v1's deleteLectures. As there, nothing is deleted unless
// every lecture is the course's, and each deletion is audited; a foreign or missing
// lecture answers 404 rather than v1's 403, as courseLecture does everywhere else.
func (a *API) DeleteLectures(ctx context.Context, req *protobuf.DeleteLecturesRequest) (*emptypb.Empty, error) {
	if len(req.GetStreamIds()) == 0 {
		return nil, e.WithStatus(http.StatusBadRequest, errors.New("no lectures given"))
	}

	streams := make([]model.Stream, 0, len(req.GetStreamIds()))
	for _, id := range req.GetStreamIds() {
		stream, err := a.courseLecture(ctx, req.GetCourseId(), id)
		if err != nil {
			return nil, err
		}
		streams = append(streams, stream)
	}

	course, err := a.dao.GetCourseById(ctx, uint(req.GetCourseId()))
	if err != nil {
		return nil, e.FromGorm(err, "can't find course")
	}

	for _, stream := range streams {
		a.audit(ctx, model.AuditStreamDelete,
			fmt.Sprintf("'%s': %s (%d)", course.Name, stream.Start.Format("2006 02 Jan, 15:04"), stream.ID))
		a.dao.StreamsDao.DeleteStream(fmt.Sprintf("%d", stream.ID))
	}
	return &emptypb.Empty{}, nil
}

// DeleteLectureSeries replaces v1's deleteLectureSeries, audited the same way and
// narrowed to the course as UpdateLectureSeries is.
func (a *API) DeleteLectureSeries(ctx context.Context, req *protobuf.DeleteLectureSeriesRequest) (*emptypb.Empty, error) {
	stream, err := a.courseLecture(ctx, req.GetCourseId(), req.GetStreamId())
	if err != nil {
		return nil, err
	}
	if stream.SeriesIdentifier == "" {
		return nil, e.WithStatus(http.StatusBadRequest, errNotInSeries)
	}

	course, err := a.dao.GetCourseById(ctx, stream.CourseID)
	if err != nil {
		return nil, e.FromGorm(err, "can't find course")
	}

	a.audit(ctx, model.AuditStreamDelete,
		fmt.Sprintf("'%s': %s (%d and series)", course.Name, stream.Start.Format("2006 02 Jan, 15:04"), stream.ID))
	if err := a.dao.StreamsDao.DeleteCourseLectureSeries(stream.CourseID, stream.SeriesIdentifier); err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}
	return &emptypb.Empty{}, nil
}

// CopyLecture replaces v1's copyStream. The policy has checked the source course;
// this checks the caller administers the target too, answering 404 as
// authorizeCourseAdmin does so the status cannot tell a missing course from someone
// else's.
//
// v1's move deleted the wrong lecture: it reset the ID of the stream it then copied,
// so after CreateStream filled in the copy's ID, deleting "the original" deleted the
// copy. It also went on to delete after a failed create. Here the original's ID is
// taken first, and a failed create stops before anything is deleted.
func (a *API) CopyLecture(ctx context.Context, req *protobuf.CopyLectureRequest) (*protobuf.CopyLectureResponse, error) {
	original, err := a.courseLecture(ctx, req.GetCourseId(), req.GetStreamId())
	if err != nil {
		return nil, err
	}
	originalID := original.ID

	user, err := a.getCurrent(ctx)
	if err != nil {
		return nil, e.WithStatus(http.StatusUnauthorized, err)
	}
	target, err := a.dao.GetCourseById(ctx, uint(req.GetTargetCourseId()))
	if err != nil {
		return nil, e.FromGorm(err, "can't find target course")
	}
	if target.ID == 0 || !user.CanAdminister(target) {
		return nil, e.WithStatus(http.StatusNotFound, errors.New("no such target course"))
	}
	if req.GetMove() && target.ID == original.CourseID {
		return nil, e.WithStatus(http.StatusBadRequest, errors.New("the lecture is already in that course"))
	}

	copied := copyOfLecture(original, target.ID, req.GetMove())
	if err := a.dao.StreamsDao.CreateStream(&copied); err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}

	if req.GetMove() {
		a.audit(ctx, model.AuditStreamDelete,
			fmt.Sprintf("%d: moved to course %d as %d", originalID, target.ID, copied.ID))
		a.dao.StreamsDao.DeleteStream(fmt.Sprintf("%d", originalID))
	}
	return &protobuf.CopyLectureResponse{StreamId: uint32(copied.ID)}, nil
}

// copyOfLecture is a new lecture in course carrying stream's data.
//
// The children get fresh rows: given rows with IDs, gorm's create upserts them and
// re-points their stream_id, which is how v1's copy took the sections and files away
// from the original. New slices too, because GetStreamByID hands out its cached value
// and the original's must not change underneath it.
//
// A copy gets its own stream key, since the key is what ingest finds a lecture by. A
// move keeps it: the original is going away, and the self-streamer's settings keep
// working.
func copyOfLecture(stream model.Stream, courseID uint, move bool) model.Stream {
	c := stream
	c.Model = gorm.Model{}
	c.CourseID = courseID
	if !move {
		c.StreamKey = strings.ReplaceAll(uuid.NewV4().String(), "-", "")
	}

	c.VideoSections = make([]model.VideoSection, len(stream.VideoSections))
	for i, v := range stream.VideoSections {
		v.Model, v.StreamID = gorm.Model{}, 0
		c.VideoSections[i] = v
	}
	c.Files = make([]model.File, len(stream.Files))
	for i, f := range stream.Files {
		f.Model, f.StreamID = gorm.Model{}, 0
		c.Files[i] = f
	}
	c.Units = make([]model.StreamUnit, len(stream.Units))
	for i, u := range stream.Units {
		u.Model, u.StreamID = gorm.Model{}, 0
		c.Units[i] = u
	}
	c.Silences = make([]model.Silence, len(stream.Silences))
	for i, s := range stream.Silences {
		s.Model, s.StreamID = gorm.Model{}, 0
		c.Silences[i] = s
	}
	// Not loaded by GetStreamByID, and per-viewer or per-run besides.
	c.Chats, c.Stats, c.StreamProgresses, c.StreamWorkers, c.TranscodingProgresses = nil, nil, nil, nil, nil
	return c
}
