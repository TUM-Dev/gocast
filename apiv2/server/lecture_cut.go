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
	"github.com/TUM-Dev/gocast/pkg/storagelimit"
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
	// Cut asks a runner to cut one version of the VOD of sourceStreamID. The result is the VOD
	// of targetStreamID, which is the same stream to replace the VOD.
	Cut(sourceStreamID, targetStreamID uint, source CutSource, discard []model.CutRange) error
	// ScanSilences asks a runner to detect the silences of the stream again.
	ScanSilences(streamID uint) error
}

// WithCutting lets the cut editor's RPCs reach a runner. Without it they answer 503.
func WithCutting(c Cutting) Option {
	return func(a *API) { a.cutting = c }
}

var (
	errNoCutRunner      = errors.New("no runner is available to take the job")
	errNoWaveform       = errors.New("the lecture's waveform has not been built yet")
	errVersionsDiffer   = errors.New("the versions of the recording differ in length by more than 60 seconds")
	errCutBusy          = errors.New("the lecture is being converted")
	errCutOpen          = errors.New("an earlier cut of the lecture is not finished: retry or abandon it")
	errNoOpenCut        = errors.New("the lecture has no cut that is not finished")
	errNothingToRetry   = errors.New("no version of the cut failed")
	errCutPartlyStarted = errors.New("only some versions could be started: retry the cut to start the rest")
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
	b, err := storagelimit.ReadFile(file.Path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, storageReadError(err, err.Error())
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
	prevEnd := 0.0
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
		prevEnd = end
	}
	ranges = snapCut(ranges, length)
	removed := 0.0
	for _, r := range ranges {
		removed += math.Min(r.End, length) - r.Start
	}
	if length-removed < 1 {
		return nil, e.WithStatus(http.StatusBadRequest, errors.New("the cut leaves less than a second of the recording"))
	}
	return ranges, nil
}

// minKeepSeconds is the shortest stretch the runner keeps, see runner/pkg/actions/cutvod.go.
const minKeepSeconds = 0.1

// snapCut folds the stretches between ranges, and before the first and after the last, that
// are shorter than the runner keeps into the cut. Left out by the runner and not listed in the
// ranges, they would be gone from the video while the times of the lecture keep counting them.
func snapCut(ranges []model.CutRange, length float64) []model.CutRange {
	out := make([]model.CutRange, 0, len(ranges))
	for _, r := range ranges {
		if last := len(out) - 1; last >= 0 && r.Start-out[last].End < minKeepSeconds {
			out[last].End = math.Max(out[last].End, r.End)
			continue
		}
		out = append(out, r)
	}
	if out[0].Start < minKeepSeconds {
		out[0].Start = 0
	}
	if last := &out[len(out)-1]; length-last.End < minKeepSeconds {
		last.End = math.Max(last.End, length)
	}
	return out
}

// CutLecture has the runners cut the discarded ranges out of every version of the lecture's
// recording, in place or into a copy of the lecture.
//
// The versions are cut by one job each, and what moves with the times of the recording,
// the sections and offsets, only once all of them are done, see runner_manager. The cut is
// kept until then, with its ranges. A version that fails leaves itself as it was and is
// listed as failed, and RetryLectureCut cuts just those again; AbandonLectureCut gives up.
// Starting one is claimed in the database, so a lecture has at most one cut at a time.
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

	// A conversion that is under way, an upload say, owns the files. One that failed does not.
	rows, err := a.cutProgress(stream.ID)
	if err != nil {
		return nil, err
	}
	if slices.ContainsFunc(rows, func(p model.TranscodingProgress) bool { return !p.Failed }) {
		return nil, e.WithStatus(http.StatusConflict, errCutBusy)
	}
	// Said before the waveform is asked for, which a cut that is not finished has taken away
	// and which nobody needs to build to cut again. Claiming below is what makes it atomic.
	if _, err := a.dao.LectureCutDao.GetLectureCutBySource(stream.ID); err == nil {
		return nil, e.WithStatus(http.StatusConflict, errCutOpen)
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
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
	// Taken back if the cut does not start, so that no empty copy is left behind. Before the
	// cut is claimed this is all there is to take back: the progress of the lecture is
	// someone else's, as a cut that is not finished keeps its failed versions there.
	dropCopy := func() {
		if !duplicate {
			return
		}
		if err := a.dao.StreamsDao.RemoveStream(fmt.Sprintf("%d", target.ID)); err != nil {
			a.log.Warn("can't remove the copy of a cut that wasn't started", "stream", target.ID, "err", err)
		}
	}

	cut := model.LectureCut{TargetStreamID: target.ID, SourceStreamID: stream.ID, SourcePrivate: stream.Private, Discard: discard, Token: model.NewCutToken()}
	claimed, err := a.dao.LectureCutDao.ClaimLectureCut(cut)
	if err != nil {
		dropCopy()
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}
	if !claimed {
		dropCopy()
		return nil, e.WithStatus(http.StatusConflict, errCutOpen)
	}
	// A version the lecture has no recording of, whose upload once failed, would keep an
	// in-place cut open for good: nothing can be retried for it. For a copy they are the
	// source's, and not in the way.
	if !duplicate {
		for _, p := range rows {
			if slices.Contains(versions, p.Version) {
				continue
			}
			if err := a.dao.StreamsDao.RemoveTranscodingProgress(p.Version, target.ID); err != nil {
				a.log.Warn("can't clear the progress of a version that isn't cut", "stream", target.ID, "err", err)
			}
		}
	}
	// Once it is claimed the rows are this cut's, and go with it.
	dropCut := func() {
		if err := a.dao.LectureCutDao.DeleteLectureCut(target.ID); err != nil {
			a.log.Warn("can't drop a cut that wasn't started", "stream", target.ID, "err", err)
		}
		for _, v := range versions {
			if err := a.dao.StreamsDao.RemoveTranscodingProgress(v, target.ID); err != nil {
				a.log.Warn("can't clear the progress of a cut that wasn't started", "stream", target.ID, "err", err)
			}
		}
		dropCopy()
	}
	// The card shows the lecture as converting from here on, see proxyLectureMedia. Not
	// optional, unlike for an upload: the cut is finished when no version has a row left, so
	// a version without one would have it finished early.
	for _, v := range versions {
		if err := a.dao.StreamsDao.SaveTranscodingProgress(model.TranscodingProgress{StreamID: target.ID, Version: v}); err != nil {
			dropCut()
			return nil, e.WithStatus(http.StatusInternalServerError, err)
		}
	}

	started, err := a.startVersions(stream.ID, target.ID, sources, discard)
	if err != nil {
		if started == 0 {
			// Nothing was handed over, so nothing has changed.
			dropCut()
			return nil, e.WithStatus(http.StatusServiceUnavailable, errNoCutRunner)
		}
		// Those that were are cut, and the rest are listed as failed for a retry.
		return nil, e.WithStatus(http.StatusServiceUnavailable, errCutPartlyStarted)
	}

	mode := "in place"
	if duplicate {
		mode = fmt.Sprintf("into %d", target.ID)
	}
	a.audit(ctx, model.AuditStreamEdit, fmt.Sprintf("%d: %d ranges cut out %s", stream.ID, len(discard), mode), forStream(stream))
	return &protobuf.CutLectureResponse{StreamId: uint32(target.ID)}, nil
}

// startVersions hands the versions to the runners one by one, and returns how many it got
// to. When a runner takes one, it is not asked again: its job goes on, and its version
// finishes or fails like any other. The version that was refused and the ones after it are
// listed as failed, which is what a retry picks up.
func (a *API) startVersions(sourceID, targetID uint, sources []CutSource, discard []model.CutRange) (started int, err error) {
	for i, src := range sources {
		if err := a.cutting.Cut(sourceID, targetID, src, discard); err != nil {
			a.log.Warn("can't hand a cut to the runners", "stream", sourceID, "target", targetID, "version", src.Version, "err", err)
			for _, rest := range sources[i:] {
				failed := model.TranscodingProgress{StreamID: targetID, Version: rest.Version, Failed: true, Error: "The cut could not be handed to a runner."}
				if serr := a.dao.StreamsDao.SaveTranscodingProgress(failed); serr != nil {
					a.log.Warn("can't mark the cut as failed", "stream", targetID, "version", rest.Version, "err", serr)
				}
			}
			return i, err
		}
	}
	return len(sources), nil
}

// cutProgress is the progress rows a stream has, of the versions that are being converted
// or failed.
func (a *API) cutProgress(streamID uint) ([]model.TranscodingProgress, error) {
	var rows []model.TranscodingProgress
	for _, v := range cutVersions {
		p, err := a.dao.StreamsDao.GetTranscodingProgressByVersion(v, streamID)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			continue
		}
		if err != nil {
			return nil, e.WithStatus(http.StatusInternalServerError, err)
		}
		rows = append(rows, p)
	}
	return rows, nil
}

// openCut is the cut of a lecture that is not finished, 404 if there is none.
func (a *API) openCut(sourceID uint) (model.LectureCut, error) {
	cut, err := a.dao.LectureCutDao.GetLectureCutBySource(sourceID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return model.LectureCut{}, e.WithStatus(http.StatusNotFound, errNoOpenCut)
	}
	if err != nil {
		return model.LectureCut{}, e.WithStatus(http.StatusInternalServerError, err)
	}
	return cut, nil
}

// GetLectureCut tells whether a cut of the lecture is not finished, and how far its versions are.
func (a *API) GetLectureCut(ctx context.Context, req *protobuf.GetLectureCutRequest) (*protobuf.GetLectureCutResponse, error) {
	stream, err := a.courseLecture(ctx, req.GetCourseId(), req.GetStreamId())
	if err != nil {
		return nil, err
	}
	cut, err := a.dao.LectureCutDao.GetLectureCutBySource(stream.ID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return &protobuf.GetLectureCutResponse{}, nil
	}
	if err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}
	rows, err := a.cutProgress(cut.TargetStreamID)
	if err != nil {
		return nil, err
	}
	progresses := make([]*protobuf.LectureTranscodingProgress, 0, len(rows))
	for _, p := range rows {
		progresses = append(progresses, &protobuf.LectureTranscodingProgress{
			Version: string(p.Version), Progress: int32(p.Progress), Failed: p.Failed, Error: p.Error,
		})
	}
	return &protobuf.GetLectureCutResponse{
		Open: true, TargetStreamId: uint32(cut.TargetStreamID), Duplicate: cut.Duplicate(), Progresses: progresses,
	}, nil
}

// RetryLectureCut cuts the versions of an open cut that failed, with the ranges it was made
// with. The versions that are done are left alone: for a replaced recording they were cut
// already, and cutting them again would cut the ranges out of what is left.
func (a *API) RetryLectureCut(ctx context.Context, req *protobuf.RetryLectureCutRequest) (*emptypb.Empty, error) {
	stream, err := a.courseLecture(ctx, req.GetCourseId(), req.GetStreamId())
	if err != nil {
		return nil, err
	}
	cut, err := a.openCut(stream.ID)
	if err != nil {
		return nil, err
	}
	if a.cutting == nil {
		return nil, e.WithStatus(http.StatusServiceUnavailable, errNoCutRunner)
	}
	rows, err := a.cutProgress(cut.TargetStreamID)
	if err != nil {
		return nil, err
	}
	var failed []model.StreamVersion
	for _, p := range rows {
		if !p.Failed {
			return nil, e.WithStatus(http.StatusConflict, errCutBusy)
		}
		failed = append(failed, p.Version)
	}
	if len(failed) == 0 {
		return nil, e.WithStatus(http.StatusConflict, errNothingToRetry)
	}

	all, err := cutSources(stream)
	if err != nil {
		return nil, err
	}
	var sources []CutSource
	for _, s := range all {
		if slices.Contains(failed, s.Version) {
			sources = append(sources, s)
		}
	}
	if len(sources) == 0 {
		return nil, e.WithStatus(http.StatusConflict, errNothingToRetry)
	}
	for _, s := range sources {
		if err := a.dao.StreamsDao.SaveTranscodingProgress(model.TranscodingProgress{StreamID: cut.TargetStreamID, Version: s.Version}); err != nil {
			return nil, e.WithStatus(http.StatusInternalServerError, err)
		}
	}
	if _, err := a.startVersions(stream.ID, cut.TargetStreamID, sources, cut.Discard); err != nil {
		return nil, e.WithStatus(http.StatusServiceUnavailable, errNoCutRunner)
	}
	a.audit(ctx, model.AuditStreamEdit, fmt.Sprintf("%d: cut retried for %d versions", stream.ID, len(sources)), forStream(stream))
	return &emptypb.Empty{}, nil
}

// AbandonLectureCut gives up an open cut that has no version still being cut.
//
// A new lecture that was made for it is removed, and nothing else has changed. A replaced
// recording can't be put back: the versions that were cut stay cut. If the combined version
// is one of them, the lecture's sections and trimming move with it, as they would have when
// the cut finished. The versions that failed are left as they were, and the answer names them.
func (a *API) AbandonLectureCut(ctx context.Context, req *protobuf.AbandonLectureCutRequest) (*protobuf.AbandonLectureCutResponse, error) {
	stream, err := a.courseLecture(ctx, req.GetCourseId(), req.GetStreamId())
	if err != nil {
		return nil, err
	}
	cut, err := a.openCut(stream.ID)
	if err != nil {
		return nil, err
	}
	rows, err := a.cutProgress(cut.TargetStreamID)
	if err != nil {
		return nil, err
	}
	var failed []model.StreamVersion
	for _, p := range rows {
		if !p.Failed {
			return nil, e.WithStatus(http.StatusConflict, errCutBusy)
		}
		failed = append(failed, p.Version)
	}
	clearRows := func() {
		for _, v := range failed {
			if err := a.dao.StreamsDao.RemoveTranscodingProgress(v, cut.TargetStreamID); err != nil {
				a.log.Warn("can't clear the progress of an abandoned cut", "stream", cut.TargetStreamID, "err", err)
			}
		}
	}

	resp := &protobuf.AbandonLectureCutResponse{}
	if cut.Duplicate() {
		clearRows()
		if err := a.dao.StreamsDao.RemoveStream(fmt.Sprintf("%d", cut.TargetStreamID)); err != nil {
			return nil, e.WithStatus(http.StatusInternalServerError, err)
		}
		if err := a.dao.LectureCutDao.DeleteLectureCut(cut.TargetStreamID); err != nil {
			return nil, e.WithStatus(http.StatusInternalServerError, err)
		}
	} else {
		recorded := recordedVersions(stream)
		anyCut := len(recorded) > len(failed)
		if slices.Contains(recorded, model.COMB) && !slices.Contains(failed, model.COMB) {
			if _, err := a.dao.LectureCutDao.FinishLectureCut(cut); err != nil {
				return nil, e.WithStatus(http.StatusInternalServerError, err)
			}
			if err := a.cutting.ScanSilences(stream.ID); err != nil {
				a.log.Warn("can't request the silence scan of a cut lecture", "stream", stream.ID, "err", err)
			}
		} else if err := a.dao.LectureCutDao.DeleteLectureCut(cut.TargetStreamID); err != nil {
			return nil, e.WithStatus(http.StatusInternalServerError, err)
		}
		clearRows()
		// With nothing cut, nothing differs between the versions.
		if anyCut {
			for _, v := range failed {
				resp.LeftUncut = append(resp.LeftUncut, string(v))
			}
		}
	}
	a.audit(ctx, model.AuditStreamEdit, fmt.Sprintf("%d: cut abandoned, %d versions left uncut", stream.ID, len(resp.LeftUncut)), forStream(stream))
	return resp, nil
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
