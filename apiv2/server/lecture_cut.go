package apiv2

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"os"
	"slices"

	"google.golang.org/protobuf/types/known/emptypb"
	"gorm.io/gorm"

	e "github.com/TUM-Dev/gocast/apiv2/errors"
	protobuf "github.com/TUM-Dev/gocast/apiv2/protobuf/server"
	"github.com/TUM-Dev/gocast/model"
)

// CutSource is one version of a VOD and the signed playlist a runner reads it from. The
// runner fetches it from wherever the edge serves it, it does not look on its own disk.
type CutSource struct {
	Version     model.StreamVersion
	PlaylistURL string
}

// Cutting hands the cut editor's work to the runners. See WithCutting.
type Cutting interface {
	// RequestWaveform asks a runner for the audio level timeline of a stream and the length
	// of each of the versions. The answer is stored when it comes.
	RequestWaveform(streamID uint, sources []CutSource) error
	// Cut asks runners to cut each of the versions of the VOD of sourceStreamID, one job
	// each. The result is the VOD of targetStreamID, which is the same stream to replace
	// the VOD.
	Cut(sourceStreamID, targetStreamID uint, sources []CutSource, discard []model.CutRange) error
}

// WithCutting lets the cut editor's RPCs reach a runner. Without it they answer 503.
func WithCutting(c Cutting) Option {
	return func(a *API) { a.cutting = c }
}

var (
	errNoCutRunner    = errors.New("no runner is available to take the job")
	errNoWaveform     = errors.New("the lecture's waveform has not been built yet")
	errVersionsDiffer = errors.New("the versions of the recording differ in length by more than 30 seconds")
	errCutBusy        = errors.New("the lecture is being converted")
)

// recordedVersions lists the versions of a lecture that have a VOD, best first.
func recordedVersions(s model.Stream) []model.StreamVersion {
	var out []model.StreamVersion
	for _, v := range []struct {
		url     string
		version model.StreamVersion
	}{{s.PlaylistUrl, model.COMB}, {s.PlaylistUrlCAM, model.CAM}, {s.PlaylistUrlPRES, model.PRES}} {
		if v.url != "" {
			out = append(out, v.version)
		}
	}
	return out
}

// cutSources signs the playlists of the lecture's versions for the runners to read. Signing
// changes the playlist fields it is given, so it gets a copy of the cached stream.
func cutSources(stream model.Stream) ([]CutSource, error) {
	signed := stream
	if err := signPlaylists(&signed, &model.User{}, false); err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, fmt.Errorf("sign playlists: %w", err))
	}
	var out []CutSource
	for _, v := range []CutSource{
		{model.COMB, signed.PlaylistUrl}, {model.CAM, signed.PlaylistUrlCAM}, {model.PRES, signed.PlaylistUrlPRES},
	} {
		if v.PlaylistURL != "" {
			out = append(out, v)
		}
	}
	return out, nil
}

// cuttable is the versions of a lecture that can be cut, or why it can't be.
func cuttable(s model.Stream) ([]model.StreamVersion, error) {
	versions := recordedVersions(s)
	if !s.Recording || s.LiveNow || len(versions) == 0 {
		return nil, e.WithStatus(http.StatusBadRequest, errNoRecording)
	}
	return versions, nil
}

// RequestLectureWaveform has a runner build the timeline the cut editor draws.
func (a *API) RequestLectureWaveform(ctx context.Context, req *protobuf.RequestLectureWaveformRequest) (*emptypb.Empty, error) {
	stream, err := a.courseLecture(ctx, req.GetCourseId(), req.GetStreamId())
	if err != nil {
		return nil, err
	}
	if _, err := cuttable(stream); err != nil {
		return nil, err
	}
	if a.cutting == nil {
		return nil, e.WithStatus(http.StatusServiceUnavailable, errNoCutRunner)
	}
	sources, err := cutSources(stream)
	if err != nil {
		return nil, err
	}
	if err := a.cutting.RequestWaveform(stream.ID, sources); err != nil {
		a.log.Warn("can't request a waveform", "stream", stream.ID, "err", err)
		return nil, e.WithStatus(http.StatusServiceUnavailable, errNoCutRunner)
	}
	return &emptypb.Empty{}, nil
}

// lectureWaveform is the stored timeline of the stream, or nil if a runner has not delivered
// one.
func (a *API) lectureWaveform(streamID uint) (*model.Waveform, error) {
	file, err := a.dao.FileDao.GetStreamFile(streamID, model.FileType(model.FILETYPE_WAVEFORM))
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}
	b, err := os.ReadFile(file.Path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}
	var w model.Waveform
	if err := json.Unmarshal(b, &w); err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, fmt.Errorf("waveform of stream %d: %w", streamID, err))
	}
	return &w, nil
}

// GetLectureWaveform is the timeline of the lecture, once a runner has built it.
func (a *API) GetLectureWaveform(ctx context.Context, req *protobuf.GetLectureWaveformRequest) (*protobuf.GetLectureWaveformResponse, error) {
	stream, err := a.courseLecture(ctx, req.GetCourseId(), req.GetStreamId())
	if err != nil {
		return nil, err
	}
	w, err := a.lectureWaveform(stream.ID)
	if err != nil {
		return nil, err
	}
	if w == nil {
		return &protobuf.GetLectureWaveformResponse{}, nil
	}
	durations := make(map[string]float64, len(w.Durations))
	for v, d := range w.Durations {
		durations[string(v)] = d
	}
	return &protobuf.GetLectureWaveformResponse{
		Ready:            true,
		Peaks:            w.Peaks,
		BucketSeconds:    w.BucketSeconds,
		VersionDurations: durations,
		VersionsDiffer:   w.DurationGap() > model.MaxVersionDurationGap,
		Silences:         watchSilences(&stream),
	}, nil
}

// validateCut checks the ranges to cut out of a recording that is length seconds long
// in its shortest version. They must be sorted and apart, so that what the editor shows is
// what the runner does, and leave something to watch.
func validateCut(discard []*protobuf.LectureCutRange, length float64) ([]model.CutRange, error) {
	if len(discard) == 0 {
		return nil, e.WithStatus(http.StatusBadRequest, errors.New("nothing to cut"))
	}
	ranges := make([]model.CutRange, len(discard))
	removed, prevEnd := 0.0, 0.0
	for i, r := range discard {
		start, end := r.GetStart(), r.GetEnd()
		switch {
		case math.IsNaN(start) || math.IsNaN(end) || math.IsInf(start, 0) || math.IsInf(end, 0):
			return nil, e.WithStatus(http.StatusBadRequest, fmt.Errorf("range %d is not a number", i))
		case start < 0 || end <= start:
			return nil, e.WithStatus(http.StatusBadRequest, fmt.Errorf("range %d, %v to %v, is no stretch of the recording", i, start, end))
		case start < prevEnd:
			return nil, e.WithStatus(http.StatusBadRequest, fmt.Errorf("range %d starts before the one before it ends", i))
		case start >= length:
			return nil, e.WithStatus(http.StatusBadRequest, fmt.Errorf("range %d starts after the recording ends", i))
		}
		ranges[i] = model.CutRange{Start: start, End: end}
		removed += math.Min(end, length) - start
		prevEnd = end
	}
	if length-removed < 1 {
		return nil, e.WithStatus(http.StatusBadRequest, errors.New("the cut leaves less than a second of the recording"))
	}
	return ranges, nil
}

// CutLecture has the runners cut the discarded ranges out of every version of the lecture's
// recording, in place or into a copy of the lecture.
//
// The versions are cut by one job each, and what moves with the times of the recording,
// the sections and offsets, only once all of them are done, see runner_manager. A job
// that fails leaves its version as it was and the lecture's progress says so; cutting again
// replaces that.
func (a *API) CutLecture(ctx context.Context, req *protobuf.CutLectureRequest) (*protobuf.CutLectureResponse, error) {
	stream, err := a.courseLecture(ctx, req.GetCourseId(), req.GetStreamId())
	if err != nil {
		return nil, err
	}
	if req.GetMode() != protobuf.LectureCutMode_LECTURE_CUT_MODE_OVERWRITE && req.GetMode() != protobuf.LectureCutMode_LECTURE_CUT_MODE_DUPLICATE {
		return nil, e.WithStatus(http.StatusBadRequest, errors.New("mode must be OVERWRITE or DUPLICATE"))
	}
	versions, err := cuttable(stream)
	if err != nil {
		return nil, err
	}
	if a.cutting == nil {
		return nil, e.WithStatus(http.StatusServiceUnavailable, errNoCutRunner)
	}

	// A version that failed has no conversion under way, and cutting again is the retry.
	for _, v := range cutVersions {
		p, err := a.dao.StreamsDao.GetTranscodingProgressByVersion(v, stream.ID)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			continue
		}
		if err != nil {
			return nil, e.WithStatus(http.StatusInternalServerError, err)
		}
		if !p.Failed {
			return nil, e.WithStatus(http.StatusConflict, errCutBusy)
		}
	}

	w, err := a.lectureWaveform(stream.ID)
	if err != nil {
		return nil, err
	}
	if w == nil || len(w.Durations) == 0 {
		return nil, e.WithStatus(http.StatusBadRequest, errNoWaveform)
	}
	if w.DurationGap() > model.MaxVersionDurationGap {
		return nil, e.WithStatus(http.StatusBadRequest, errVersionsDiffer)
	}
	shortest := math.Inf(1)
	for _, d := range w.Durations {
		shortest = math.Min(shortest, d)
	}
	discard, err := validateCut(req.GetDiscard(), shortest)
	if err != nil {
		return nil, err
	}
	// Before anything is created: signing can fail, and a copy would be left behind.
	sources, err := cutSources(stream)
	if err != nil {
		return nil, err
	}

	duplicate := req.GetMode() == protobuf.LectureCutMode_LECTURE_CUT_MODE_DUPLICATE
	target := stream
	if duplicate {
		target = lectureForCut(stream)
		if err := a.dao.StreamsDao.CreateStream(&target); err != nil {
			return nil, e.WithStatus(http.StatusInternalServerError, err)
		}
	}
	// Taken back if the cut can't be started, so that no empty copy is left behind.
	abandon := func() {
		if !duplicate {
			return
		}
		for _, v := range versions {
			if err := a.dao.StreamsDao.RemoveTranscodingProgress(v, target.ID); err != nil {
				a.log.Warn("can't clear the progress of a cut that wasn't started", "stream", target.ID, "err", err)
			}
		}
		if err := a.dao.StreamsDao.RemoveStream(fmt.Sprintf("%d", target.ID)); err != nil {
			a.log.Warn("can't remove the copy of a cut that wasn't started", "stream", target.ID, "err", err)
		}
	}

	cut := model.LectureCut{TargetStreamID: target.ID, SourceStreamID: stream.ID, SourcePrivate: stream.Private, Discard: discard}
	if err := a.dao.LectureCutDao.SaveLectureCut(cut); err != nil {
		abandon()
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}
	// The card shows the lecture as converting from here on, see proxyLectureMedia. Not
	// optional, unlike for an upload: the cut is finished when no version has a row left, so
	// a version without one would have it finished early.
	for _, v := range versions {
		if err := a.dao.StreamsDao.SaveTranscodingProgress(model.TranscodingProgress{StreamID: target.ID, Version: v}); err != nil {
			if derr := a.dao.LectureCutDao.DeleteLectureCut(target.ID); derr != nil {
				a.log.Warn("can't drop a cut that wasn't started", "stream", target.ID, "err", derr)
			}
			for _, done := range versions {
				_ = a.dao.StreamsDao.RemoveTranscodingProgress(done, target.ID)
			}
			abandon()
			return nil, e.WithStatus(http.StatusInternalServerError, err)
		}
	}
	if err := a.cutting.Cut(stream.ID, target.ID, sources, discard); err != nil {
		a.log.Warn("can't hand a cut to the runners", "stream", stream.ID, "target", target.ID, "err", err)
		if err := a.dao.LectureCutDao.DeleteLectureCut(target.ID); err != nil {
			a.log.Warn("can't drop a cut that wasn't started", "stream", target.ID, "err", err)
		}
		abandon()
		if !duplicate {
			// Versions that were handed over before the failing one are cut regardless, and the
			// rest are not: the lecture is told so rather than left converting for good.
			for _, v := range versions {
				failed := model.TranscodingProgress{StreamID: target.ID, Version: v, Failed: true, Error: "The cut could not be handed to a runner."}
				if err := a.dao.StreamsDao.SaveTranscodingProgress(failed); err != nil {
					a.log.Warn("can't mark the cut as failed", "stream", target.ID, "err", err)
				}
			}
		}
		return nil, e.WithStatus(http.StatusServiceUnavailable, errNoCutRunner)
	}

	mode := "in place"
	if duplicate {
		mode = fmt.Sprintf("into %d", target.ID)
	}
	a.audit(ctx, model.AuditStreamEdit, fmt.Sprintf("%d: %d ranges cut out %s", stream.ID, len(discard), mode), forStream(stream))
	return &protobuf.CutLectureResponse{StreamId: uint32(target.ID)}, nil
}

// cutVersions are the versions a lecture's VODs come in.
var cutVersions = []model.StreamVersion{model.COMB, model.CAM, model.PRES}

// lectureForCut is the lecture that gets a cut copy of stream's recording: its data, but
// private and without a recording until the cut is done, and without anything that
// describes the uncut one.
func lectureForCut(stream model.Stream) model.Stream {
	c := copyOfLecture(stream, stream.CourseID, false)
	c.Private = true
	c.Recording = false
	c.PlaylistUrl, c.PlaylistUrlCAM, c.PlaylistUrlPRES = "", "", ""
	c.Duration = sql.NullInt32{}
	c.Silences = nil
	c.Files = slices.DeleteFunc(c.Files, func(f model.File) bool { return f.Type == model.FileType(model.FILETYPE_WAVEFORM) })
	return c
}
