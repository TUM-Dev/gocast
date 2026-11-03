package apiv2

import (
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"go.uber.org/mock/gomock"
	"google.golang.org/grpc/codes"
	"gorm.io/gorm"

	protobuf "github.com/TUM-Dev/gocast/apiv2/protobuf/server"
	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/mock_dao"
	"github.com/TUM-Dev/gocast/model"
)

type fakeCutting struct {
	waveformFor     []uint
	waveformSources []CutSource
	waveformErr     error
	// cutErrs fails the cut of a version, in addition to cutErr for all of them.
	cutErr  error
	cutErrs map[model.StreamVersion]error
	cuts    []cutCall
	scanned []uint
}

type cutCall struct {
	source, target uint
	src            CutSource
	discard        []model.CutRange
}

func (f *fakeCutting) RequestWaveform(streamID uint, sources []CutSource) error {
	f.waveformFor = append(f.waveformFor, streamID)
	f.waveformSources = sources
	return f.waveformErr
}

func (f *fakeCutting) Cut(source, target uint, src CutSource, discard []model.CutRange) error {
	if err := f.cutErrs[src.Version]; err != nil {
		return err
	}
	if f.cutErr != nil {
		return f.cutErr
	}
	f.cuts = append(f.cuts, cutCall{source, target, src, discard})
	return nil
}

func (f *fakeCutting) ScanSilences(streamID uint) error {
	f.scanned = append(f.scanned, streamID)
	return nil
}

// versionsCut is the versions the runners were asked to cut, in order.
func (f *fakeCutting) versionsCut() []model.StreamVersion {
	var out []model.StreamVersion
	for _, c := range f.cuts {
		out = append(out, c.src.Version)
	}
	return out
}

type cutMocks struct {
	streams *mock_dao.MockStreamsDao
	files   *mock_dao.MockFileDao
	cuts    *mock_dao.MockLectureCutDao
	cutting *fakeCutting
}

func newCutAPI(t *testing.T) (*API, cutMocks) {
	noSigning(t)
	ctrl := gomock.NewController(t)
	m := cutMocks{
		streams: mock_dao.NewMockStreamsDao(ctrl),
		files:   mock_dao.NewMockFileDao(ctrl),
		cuts:    mock_dao.NewMockLectureCutDao(ctrl),
		cutting: &fakeCutting{},
	}
	return &API{
		dao: dao.DaoWrapper{
			StreamsDao: m.streams, FileDao: m.files, LectureCutDao: m.cuts, AuditDao: auditSink(t),
		},
		log:     slog.Default(),
		cutting: m.cutting,
	}, m
}

// cuttableLecture is stream 7 with a combined and a camera VOD.
func cuttableLecture() model.Stream {
	s := courseOneLecture()
	s.Recording = true
	s.PlaylistUrl, s.PlaylistUrlCAM = "https://comb", "https://cam"
	s.Silences = []model.Silence{{Start: 100, End: 200}}
	s.Files = []model.File{
		{Model: gorm.Model{ID: 1}, Type: model.FILETYPE_ATTACHMENT, Path: "/a.pdf"},
		{Model: gorm.Model{ID: 2}, Type: model.FILETYPE_WAVEFORM, Path: "/w.json"},
	}
	return s
}

// storeWaveform writes w the way the manager does and has the file DAO find it.
func storeWaveform(t *testing.T, m cutMocks, w model.Waveform) {
	t.Helper()
	b, err := json.Marshal(w)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "7.json")
	if err := os.WriteFile(path, b, 0o600); err != nil {
		t.Fatal(err)
	}
	m.files.EXPECT().GetStreamFile(uint(7), model.FileType(model.FILETYPE_WAVEFORM)).Return(model.File{Path: path}, nil)
}

var hourLong = model.Waveform{
	BucketSeconds: 1,
	Peaks:         []byte{1, 2, 3},
	Durations:     map[model.StreamVersion]float64{model.COMB: 3600, model.CAM: 3590},
}

func discard(pairs ...float64) []*protobuf.LectureCutRange {
	var out []*protobuf.LectureCutRange
	for i := 0; i < len(pairs); i += 2 {
		out = append(out, &protobuf.LectureCutRange{Start: pairs[i], End: pairs[i+1]})
	}
	return out
}

func TestRequestLectureWaveform(t *testing.T) {
	req := &protobuf.RequestLectureWaveformRequest{CourseId: 1, StreamId: 7}

	t.Run("asks a runner", func(t *testing.T) {
		api, m := newCutAPI(t)
		m.streams.EXPECT().GetStreamByID(gomock.Any(), "7").Return(cuttableLecture(), nil)
		if _, err := api.RequestLectureWaveform(asCaller(lectureAdmin), req); err != nil {
			t.Fatal(err)
		}
		if len(m.cutting.waveformFor) != 1 || m.cutting.waveformFor[0] != 7 {
			t.Errorf("asked for %v", m.cutting.waveformFor)
		}
		// The runner reads the versions from the edge, through signed playlists.
		want := []CutSource{{model.COMB, "https://comb?jwt=signed"}, {model.CAM, "https://cam"}}
		if !reflect.DeepEqual(m.cutting.waveformSources, want) {
			t.Errorf("sources = %v, want %v", m.cutting.waveformSources, want)
		}
	})

	t.Run("400s a lecture without a recording", func(t *testing.T) {
		api, m := newCutAPI(t)
		m.streams.EXPECT().GetStreamByID(gomock.Any(), "7").Return(courseOneLecture(), nil)
		_, err := api.RequestLectureWaveform(asCaller(lectureAdmin), req)
		wantCode(t, err, codes.InvalidArgument)
	})

	t.Run("400s a live lecture", func(t *testing.T) {
		api, m := newCutAPI(t)
		live := cuttableLecture()
		live.LiveNow = true
		m.streams.EXPECT().GetStreamByID(gomock.Any(), "7").Return(live, nil)
		_, err := api.RequestLectureWaveform(asCaller(lectureAdmin), req)
		wantCode(t, err, codes.InvalidArgument)
	})

	t.Run("503s without runners", func(t *testing.T) {
		api, m := newCutAPI(t)
		api.cutting = nil
		m.streams.EXPECT().GetStreamByID(gomock.Any(), "7").Return(cuttableLecture(), nil)
		_, err := api.RequestLectureWaveform(asCaller(lectureAdmin), req)
		wantCode(t, err, codes.Unavailable)
	})

	t.Run("503s a runner that doesn't answer", func(t *testing.T) {
		api, m := newCutAPI(t)
		m.cutting.waveformErr = errors.New("no runner is available")
		m.streams.EXPECT().GetStreamByID(gomock.Any(), "7").Return(cuttableLecture(), nil)
		_, err := api.RequestLectureWaveform(asCaller(lectureAdmin), req)
		wantCode(t, err, codes.Unavailable)
	})

	t.Run("404s another course's lecture", func(t *testing.T) {
		api, m := newCutAPI(t)
		m.streams.EXPECT().GetStreamByID(gomock.Any(), "7").Return(foreignLecture(), nil)
		_, err := api.RequestLectureWaveform(asCaller(lectureAdmin), req)
		wantCode(t, err, codes.NotFound)
	})
}

func TestGetLectureWaveform(t *testing.T) {
	req := &protobuf.GetLectureWaveformRequest{CourseId: 1, StreamId: 7}

	t.Run("is not ready before a runner delivered it", func(t *testing.T) {
		api, m := newCutAPI(t)
		m.streams.EXPECT().GetStreamByID(gomock.Any(), "7").Return(cuttableLecture(), nil)
		m.files.EXPECT().GetStreamFile(uint(7), model.FileType(model.FILETYPE_WAVEFORM)).Return(model.File{}, gorm.ErrRecordNotFound)
		resp, err := api.GetLectureWaveform(asCaller(lectureAdmin), req)
		if err != nil || resp.Ready {
			t.Errorf("got %v, %v, want not ready", resp, err)
		}
	})

	t.Run("is not ready when the file is gone", func(t *testing.T) {
		api, m := newCutAPI(t)
		m.streams.EXPECT().GetStreamByID(gomock.Any(), "7").Return(cuttableLecture(), nil)
		m.files.EXPECT().GetStreamFile(uint(7), model.FileType(model.FILETYPE_WAVEFORM)).Return(model.File{Path: "/nowhere/7.json"}, nil)
		resp, err := api.GetLectureWaveform(asCaller(lectureAdmin), req)
		if err != nil || resp.Ready {
			t.Errorf("got %v, %v, want not ready", resp, err)
		}
	})

	t.Run("answers the timeline with the silences", func(t *testing.T) {
		api, m := newCutAPI(t)
		m.streams.EXPECT().GetStreamByID(gomock.Any(), "7").Return(cuttableLecture(), nil)
		storeWaveform(t, m, hourLong)
		resp, err := api.GetLectureWaveform(asCaller(lectureAdmin), req)
		if err != nil {
			t.Fatal(err)
		}
		if !resp.Ready || string(resp.Peaks) != string(hourLong.Peaks) || resp.BucketSeconds != 1 ||
			resp.VersionDurations["COMB"] != 3600 || resp.VersionDurations["CAM"] != 3590 || resp.VersionsDiffer {
			t.Errorf("resp = %v", resp)
		}
		if len(resp.Silences) != 1 || resp.Silences[0].Start != 100 || resp.Silences[0].End != 200 {
			t.Errorf("silences = %v", resp.Silences)
		}
	})

	t.Run("says when the versions differ too much", func(t *testing.T) {
		api, m := newCutAPI(t)
		m.streams.EXPECT().GetStreamByID(gomock.Any(), "7").Return(cuttableLecture(), nil)
		w := hourLong
		w.Durations = map[model.StreamVersion]float64{model.COMB: 3600, model.CAM: 3560}
		storeWaveform(t, m, w)
		resp, err := api.GetLectureWaveform(asCaller(lectureAdmin), req)
		if err != nil || !resp.VersionsDiffer {
			t.Errorf("got %v, %v, want the versions to differ", resp, err)
		}
	})

	t.Run("404s another course's lecture", func(t *testing.T) {
		api, m := newCutAPI(t)
		m.streams.EXPECT().GetStreamByID(gomock.Any(), "7").Return(foreignLecture(), nil)
		_, err := api.GetLectureWaveform(asCaller(lectureAdmin), req)
		wantCode(t, err, codes.NotFound)
	})
}

func TestValidateCut(t *testing.T) {
	for name, tc := range map[string]struct {
		in   []*protobuf.LectureCutRange
		want bool
	}{
		"one range":             {discard(10, 20), true},
		"several":               {discard(10, 20, 30, 40), true},
		"touching":              {discard(10, 20, 20, 30), true},
		"reaching past the end": {discard(90, 500), true},
		"nothing":               {nil, false},
		"empty range":           {discard(10, 10), false},
		"backwards":             {discard(20, 10), false},
		"negative":              {discard(-1, 10), false},
		"unsorted":              {discard(30, 40, 10, 20), false},
		"overlapping":           {discard(10, 30, 20, 40), false},
		"after the end":         {discard(200, 300), false},
		"everything":            {discard(0, 200), false},
		"all but half a second": {discard(0, 99.5), false},
		"not a number":          {[]*protobuf.LectureCutRange{{Start: 1, End: nan()}}, false},
	} {
		t.Run(name, func(t *testing.T) {
			got, err := validateCut(tc.in, 100)
			if (err == nil) != tc.want {
				t.Fatalf("err = %v, want ok=%v", err, tc.want)
			}
			// Ranges that touch are one.
			if err == nil && len(got) != len(tc.in) && name != "touching" {
				t.Errorf("got %v", got)
			}
			if err != nil {
				wantCode(t, err, codes.InvalidArgument)
			}
		})
	}
}

// progressRows expects the progress rows of the three versions of a stream: those in rows
// exist, the others do not.
// cutIgnoringToken matches a claimed cut by everything but its random token, which it also
// insists on having.
func cutIgnoringToken(want model.LectureCut) gomock.Matcher {
	return gomock.Cond(func(x any) bool {
		got, ok := x.(model.LectureCut)
		if !ok || got.Token == "" {
			return false
		}
		got.Token = ""
		return reflect.DeepEqual(got, want)
	})
}

func TestSnapCut(t *testing.T) {
	for name, tc := range map[string]struct {
		in, want []model.CutRange
	}{
		"apart":            {[]model.CutRange{{Start: 10, End: 20}, {Start: 30, End: 40}}, []model.CutRange{{Start: 10, End: 20}, {Start: 30, End: 40}}},
		"touching":         {[]model.CutRange{{Start: 10, End: 20}, {Start: 20, End: 30}}, []model.CutRange{{Start: 10, End: 30}}},
		"a sliver between": {[]model.CutRange{{Start: 10, End: 20}, {Start: 20.05, End: 30}}, []model.CutRange{{Start: 10, End: 30}}},
		"a sliver first":   {[]model.CutRange{{Start: 0.05, End: 20}}, []model.CutRange{{Start: 0, End: 20}}},
		"a sliver last":    {[]model.CutRange{{Start: 10, End: 99.95}}, []model.CutRange{{Start: 10, End: 100}}},
		"exactly enough":   {[]model.CutRange{{Start: 10, End: 20}, {Start: 20.1, End: 30}}, []model.CutRange{{Start: 10, End: 20}, {Start: 20.1, End: 30}}},
	} {
		t.Run(name, func(t *testing.T) {
			if got := snapCut(tc.in, 100); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func progressRows(m cutMocks, stream uint, rows map[model.StreamVersion]model.TranscodingProgress) {
	for _, v := range []model.StreamVersion{model.COMB, model.CAM, model.PRES} {
		if p, ok := rows[v]; ok {
			p.StreamID, p.Version = stream, v
			m.streams.EXPECT().GetTranscodingProgressByVersion(v, stream).Return(p, nil)
		} else {
			m.streams.EXPECT().GetTranscodingProgressByVersion(v, stream).Return(model.TranscodingProgress{}, gorm.ErrRecordNotFound)
		}
	}
}

func TestCutLecture(t *testing.T) {
	req := func(mode protobuf.LectureCutMode, d ...float64) *protobuf.CutLectureRequest {
		return &protobuf.CutLectureRequest{CourseId: 1, StreamId: 7, Mode: mode, Discard: discard(d...)}
	}
	overwrite := protobuf.LectureCutMode_LECTURE_CUT_MODE_OVERWRITE
	duplicate := protobuf.LectureCutMode_LECTURE_CUT_MODE_DUPLICATE
	// Nothing converting, and no cut that is not finished.
	idle := func(m cutMocks) {
		progressRows(m, 7, nil)
		m.cuts.EXPECT().GetLectureCutBySource(uint(7)).Return(model.LectureCut{}, gorm.ErrRecordNotFound)
	}
	signed := []CutSource{{model.COMB, "https://comb?jwt=signed"}, {model.CAM, "https://cam"}}
	saveRows := func(m cutMocks, target uint) {
		for _, v := range []model.StreamVersion{model.COMB, model.CAM} {
			m.streams.EXPECT().SaveTranscodingProgress(model.TranscodingProgress{StreamID: target, Version: v}).Return(nil)
		}
	}
	removeRows := func(m cutMocks, target uint) {
		m.streams.EXPECT().RemoveTranscodingProgress(gomock.Any(), target).Return(nil).Times(2)
	}

	t.Run("overwrites in place", func(t *testing.T) {
		api, m := newCutAPI(t)
		m.streams.EXPECT().GetStreamByID(gomock.Any(), "7").Return(cuttableLecture(), nil)
		idle(m)
		storeWaveform(t, m, hourLong)
		want := model.LectureCut{TargetStreamID: 7, SourceStreamID: 7, Discard: []model.CutRange{{Start: 60, End: 120}, {Start: 300, End: 400}}}
		m.cuts.EXPECT().ClaimLectureCut(cutIgnoringToken(want)).Return(true, nil)
		saveRows(m, 7)

		resp, err := api.CutLecture(asCaller(lectureAdmin), req(overwrite, 60, 120, 300, 400))
		if err != nil {
			t.Fatal(err)
		}
		if resp.StreamId != 7 || len(m.cutting.cuts) != 2 {
			t.Fatalf("resp = %v, cuts = %v", resp, m.cutting.cuts)
		}
		// One job per version, from the signed playlist of that version, with the same ranges.
		for i, c := range m.cutting.cuts {
			if c.source != 7 || c.target != 7 || c.src != signed[i] || !reflect.DeepEqual(c.discard, want.Discard) {
				t.Errorf("cut %d = %+v", i, c)
			}
		}
	})

	t.Run("duplicates into a private lecture without the recording", func(t *testing.T) {
		api, m := newCutAPI(t)
		source := cuttableLecture()
		source.Private = false
		m.streams.EXPECT().GetStreamByID(gomock.Any(), "7").Return(source, nil)
		idle(m)
		storeWaveform(t, m, hourLong)
		var created model.Stream
		m.streams.EXPECT().CreateStream(gomock.Any()).DoAndReturn(func(s *model.Stream) error {
			s.ID = 8
			created = *s
			return nil
		})
		m.cuts.EXPECT().ClaimLectureCut(cutIgnoringToken(model.LectureCut{
			TargetStreamID: 8, SourceStreamID: 7, SourcePrivate: false, Discard: []model.CutRange{{Start: 60, End: 120}},
		})).Return(true, nil)
		saveRows(m, 8)

		resp, err := api.CutLecture(asCaller(lectureAdmin), req(duplicate, 60, 120))
		if err != nil {
			t.Fatal(err)
		}
		if resp.StreamId != 8 || m.cutting.cuts[0].source != 7 || m.cutting.cuts[0].target != 8 {
			t.Errorf("resp = %v, cuts = %+v", resp, m.cutting.cuts)
		}
		if !created.Private || created.Recording || created.PlaylistUrl != "" || created.PlaylistUrlCAM != "" ||
			created.CourseID != 1 || created.Name != "Old" || len(created.Silences) != 0 {
			t.Errorf("copy = %+v", created)
		}
		for _, f := range created.Files {
			if f.Type == model.FILETYPE_WAVEFORM {
				t.Error("the copy has the waveform of the original")
			}
		}
		if len(created.Files) != 1 {
			t.Errorf("files = %v, want only the attachment", created.Files)
		}
	})

	t.Run("remembers that the original was private", func(t *testing.T) {
		api, m := newCutAPI(t)
		source := cuttableLecture()
		source.Private = true
		m.streams.EXPECT().GetStreamByID(gomock.Any(), "7").Return(source, nil)
		idle(m)
		storeWaveform(t, m, hourLong)
		m.streams.EXPECT().CreateStream(gomock.Any()).DoAndReturn(func(s *model.Stream) error { s.ID = 8; return nil })
		m.cuts.EXPECT().ClaimLectureCut(gomock.Any()).DoAndReturn(func(c model.LectureCut) (bool, error) {
			if !c.SourcePrivate {
				t.Error("SourcePrivate not set")
			}
			return true, nil
		})
		m.streams.EXPECT().SaveTranscodingProgress(gomock.Any()).Return(nil).Times(2)
		if _, err := api.CutLecture(asCaller(lectureAdmin), req(duplicate, 60, 120)); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("refuses a lecture that has a cut that is not finished", func(t *testing.T) {
		api, m := newCutAPI(t)
		m.streams.EXPECT().GetStreamByID(gomock.Any(), "7").Return(cuttableLecture(), nil)
		idle(m)
		storeWaveform(t, m, hourLong)
		m.cuts.EXPECT().ClaimLectureCut(gomock.Any()).Return(false, nil)

		_, err := api.CutLecture(asCaller(lectureAdmin), req(overwrite, 60, 120))
		wantCode(t, err, codes.AlreadyExists)
		if len(m.cutting.cuts) != 0 {
			t.Error("a runner was asked")
		}
	})

	t.Run("says a cut is not finished before it asks for a waveform", func(t *testing.T) {
		api, m := newCutAPI(t)
		m.streams.EXPECT().GetStreamByID(gomock.Any(), "7").Return(cuttableLecture(), nil)
		progressRows(m, 7, map[model.StreamVersion]model.TranscodingProgress{model.CAM: failedRow(7, model.CAM)})
		m.cuts.EXPECT().GetLectureCutBySource(uint(7)).Return(openOverwrite, nil)
		// the waveform went when the version that was cut was ready: no GetStreamFile

		_, err := api.CutLecture(asCaller(lectureAdmin), req(overwrite, 60, 120))
		wantCode(t, err, codes.AlreadyExists)
		if !strings.Contains(err.Error(), "not finished") {
			t.Errorf("err = %v, want it to say that the cut is not finished", err)
		}
	})

	t.Run("takes back the copy of a duplicate that was refused", func(t *testing.T) {
		api, m := newCutAPI(t)
		m.streams.EXPECT().GetStreamByID(gomock.Any(), "7").Return(cuttableLecture(), nil)
		idle(m)
		storeWaveform(t, m, hourLong)
		m.streams.EXPECT().CreateStream(gomock.Any()).DoAndReturn(func(s *model.Stream) error { s.ID = 8; return nil })
		m.cuts.EXPECT().ClaimLectureCut(gomock.Any()).Return(false, nil)
		// only the copy goes: the lecture's own rows belong to the cut that is open
		m.streams.EXPECT().RemoveStream("8").Return(nil)

		_, err := api.CutLecture(asCaller(lectureAdmin), req(duplicate, 60, 120))
		wantCode(t, err, codes.AlreadyExists)
	})

	t.Run("takes back a duplicate that no runner took", func(t *testing.T) {
		api, m := newCutAPI(t)
		m.cutting.cutErr = errors.New("no runner is available")
		m.streams.EXPECT().GetStreamByID(gomock.Any(), "7").Return(cuttableLecture(), nil)
		idle(m)
		storeWaveform(t, m, hourLong)
		m.streams.EXPECT().CreateStream(gomock.Any()).DoAndReturn(func(s *model.Stream) error { s.ID = 8; return nil })
		m.cuts.EXPECT().ClaimLectureCut(gomock.Any()).Return(true, nil)
		// two rows when it starts, and both marked failed when no runner takes them
		m.streams.EXPECT().SaveTranscodingProgress(gomock.Any()).Return(nil).Times(4)
		m.cuts.EXPECT().DeleteLectureCut(uint(8)).Return(nil)
		removeRows(m, 8)
		m.streams.EXPECT().RemoveStream("8").Return(nil)

		_, err := api.CutLecture(asCaller(lectureAdmin), req(duplicate, 60, 120))
		wantCode(t, err, codes.Unavailable)
	})

	t.Run("leaves nothing behind when no runner took an overwrite", func(t *testing.T) {
		api, m := newCutAPI(t)
		m.cutting.cutErr = errors.New("no runner is available")
		m.streams.EXPECT().GetStreamByID(gomock.Any(), "7").Return(cuttableLecture(), nil)
		idle(m)
		storeWaveform(t, m, hourLong)
		m.cuts.EXPECT().ClaimLectureCut(gomock.Any()).Return(true, nil)
		m.streams.EXPECT().SaveTranscodingProgress(gomock.Any()).Return(nil).Times(4)
		m.cuts.EXPECT().DeleteLectureCut(uint(7)).Return(nil)
		removeRows(m, 7)

		_, err := api.CutLecture(asCaller(lectureAdmin), req(overwrite, 60, 120))
		wantCode(t, err, codes.Unavailable)
	})

	t.Run("keeps a cut that was only partly started, to be retried", func(t *testing.T) {
		api, m := newCutAPI(t)
		m.cutting.cutErrs = map[model.StreamVersion]error{model.CAM: errors.New("runner went away")}
		m.streams.EXPECT().GetStreamByID(gomock.Any(), "7").Return(cuttableLecture(), nil)
		idle(m)
		storeWaveform(t, m, hourLong)
		m.cuts.EXPECT().ClaimLectureCut(gomock.Any()).Return(true, nil)
		saveRows(m, 7)
		// The version that was not taken is the one that is marked. The combined one carries
		// on: marking it too would have a retry cut it a second time.
		m.streams.EXPECT().SaveTranscodingProgress(gomock.Any()).DoAndReturn(func(p model.TranscodingProgress) error {
			if p.Version != model.CAM || !p.Failed {
				t.Errorf("marked %+v as failed", p)
			}
			return nil
		})
		// no DeleteLectureCut, no RemoveTranscodingProgress

		_, err := api.CutLecture(asCaller(lectureAdmin), req(overwrite, 60, 120))
		wantCode(t, err, codes.Unavailable)
		if got := m.cutting.versionsCut(); !reflect.DeepEqual(got, []model.StreamVersion{model.COMB}) {
			t.Errorf("cut = %v, want only the combined version", got)
		}
	})

	t.Run("does not cut without the markers that tell when it is done", func(t *testing.T) {
		api, m := newCutAPI(t)
		m.streams.EXPECT().GetStreamByID(gomock.Any(), "7").Return(cuttableLecture(), nil)
		idle(m)
		storeWaveform(t, m, hourLong)
		m.cuts.EXPECT().ClaimLectureCut(gomock.Any()).Return(true, nil)
		m.streams.EXPECT().SaveTranscodingProgress(gomock.Any()).Return(errors.New("db down"))
		m.cuts.EXPECT().DeleteLectureCut(uint(7)).Return(nil)
		removeRows(m, 7)

		_, err := api.CutLecture(asCaller(lectureAdmin), req(overwrite, 60, 120))
		wantCode(t, err, codes.Unknown)
		if len(m.cutting.cuts) != 0 {
			t.Error("a runner was asked")
		}
	})

	t.Run("refuses while a version is converting", func(t *testing.T) {
		api, m := newCutAPI(t)
		m.streams.EXPECT().GetStreamByID(gomock.Any(), "7").Return(cuttableLecture(), nil)
		progressRows(m, 7, map[model.StreamVersion]model.TranscodingProgress{model.COMB: {Progress: 10}})
		_, err := api.CutLecture(asCaller(lectureAdmin), req(overwrite, 60, 120))
		wantCode(t, err, codes.AlreadyExists)
	})

	t.Run("goes on after a conversion that failed", func(t *testing.T) {
		api, m := newCutAPI(t)
		m.streams.EXPECT().GetStreamByID(gomock.Any(), "7").Return(cuttableLecture(), nil)
		progressRows(m, 7, map[model.StreamVersion]model.TranscodingProgress{model.COMB: {Failed: true}})
		m.cuts.EXPECT().GetLectureCutBySource(uint(7)).Return(model.LectureCut{}, gorm.ErrRecordNotFound)
		storeWaveform(t, m, hourLong)
		m.cuts.EXPECT().ClaimLectureCut(gomock.Any()).Return(true, nil)
		saveRows(m, 7)
		if _, err := api.CutLecture(asCaller(lectureAdmin), req(overwrite, 60, 120)); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("clears a failed row of a version it has no recording of", func(t *testing.T) {
		api, m := newCutAPI(t)
		m.streams.EXPECT().GetStreamByID(gomock.Any(), "7").Return(cuttableLecture(), nil)
		progressRows(m, 7, map[model.StreamVersion]model.TranscodingProgress{model.PRES: {Failed: true}})
		m.cuts.EXPECT().GetLectureCutBySource(uint(7)).Return(model.LectureCut{}, gorm.ErrRecordNotFound)
		storeWaveform(t, m, hourLong)
		m.cuts.EXPECT().ClaimLectureCut(gomock.Any()).Return(true, nil)
		m.streams.EXPECT().RemoveTranscodingProgress(model.PRES, uint(7)).Return(nil)
		saveRows(m, 7)
		if _, err := api.CutLecture(asCaller(lectureAdmin), req(overwrite, 60, 120)); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("refuses versions that differ by more than 30 seconds", func(t *testing.T) {
		api, m := newCutAPI(t)
		m.streams.EXPECT().GetStreamByID(gomock.Any(), "7").Return(cuttableLecture(), nil)
		idle(m)
		w := hourLong
		w.Durations = map[model.StreamVersion]float64{model.COMB: 3600, model.CAM: 3569}
		storeWaveform(t, m, w)
		_, err := api.CutLecture(asCaller(lectureAdmin), req(overwrite, 60, 120))
		wantCode(t, err, codes.InvalidArgument)
		if len(m.cutting.cuts) != 0 {
			t.Error("a runner was asked")
		}
	})

	t.Run("accepts versions that differ by 30 seconds", func(t *testing.T) {
		api, m := newCutAPI(t)
		m.streams.EXPECT().GetStreamByID(gomock.Any(), "7").Return(cuttableLecture(), nil)
		idle(m)
		w := hourLong
		w.Durations = map[model.StreamVersion]float64{model.COMB: 3600, model.CAM: 3570}
		storeWaveform(t, m, w)
		m.cuts.EXPECT().ClaimLectureCut(gomock.Any()).Return(true, nil)
		saveRows(m, 7)
		if _, err := api.CutLecture(asCaller(lectureAdmin), req(overwrite, 60, 120)); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("refuses without a waveform", func(t *testing.T) {
		api, m := newCutAPI(t)
		m.streams.EXPECT().GetStreamByID(gomock.Any(), "7").Return(cuttableLecture(), nil)
		idle(m)
		m.files.EXPECT().GetStreamFile(uint(7), model.FileType(model.FILETYPE_WAVEFORM)).Return(model.File{}, gorm.ErrRecordNotFound)
		_, err := api.CutLecture(asCaller(lectureAdmin), req(overwrite, 60, 120))
		wantCode(t, err, codes.InvalidArgument)
	})

	t.Run("refuses an invalid cut before doing anything", func(t *testing.T) {
		api, m := newCutAPI(t)
		m.streams.EXPECT().GetStreamByID(gomock.Any(), "7").Return(cuttableLecture(), nil)
		idle(m)
		storeWaveform(t, m, hourLong)
		_, err := api.CutLecture(asCaller(lectureAdmin), req(duplicate, 120, 60))
		wantCode(t, err, codes.InvalidArgument)
	})

	t.Run("refuses an unset mode", func(t *testing.T) {
		api, m := newCutAPI(t)
		m.streams.EXPECT().GetStreamByID(gomock.Any(), "7").Return(cuttableLecture(), nil)
		_, err := api.CutLecture(asCaller(lectureAdmin), req(protobuf.LectureCutMode_LECTURE_CUT_MODE_UNSPECIFIED, 60, 120))
		wantCode(t, err, codes.InvalidArgument)
	})

	t.Run("503s without runners", func(t *testing.T) {
		api, m := newCutAPI(t)
		api.cutting = nil
		m.streams.EXPECT().GetStreamByID(gomock.Any(), "7").Return(cuttableLecture(), nil)
		_, err := api.CutLecture(asCaller(lectureAdmin), req(overwrite, 60, 120))
		wantCode(t, err, codes.Unavailable)
	})

	t.Run("404s another course's lecture", func(t *testing.T) {
		api, m := newCutAPI(t)
		m.streams.EXPECT().GetStreamByID(gomock.Any(), "7").Return(foreignLecture(), nil)
		_, err := api.CutLecture(asCaller(lectureAdmin), req(overwrite, 60, 120))
		wantCode(t, err, codes.NotFound)
	})
}

var (
	// openOverwrite is the cut of stream 7 into itself, and openDuplicate the one into 8.
	openOverwrite = model.LectureCut{TargetStreamID: 7, SourceStreamID: 7, Discard: []model.CutRange{{Start: 60, End: 120}}}
	openDuplicate = model.LectureCut{TargetStreamID: 8, SourceStreamID: 7, Discard: []model.CutRange{{Start: 60, End: 120}}}
)

func failedRow(stream uint, v model.StreamVersion) model.TranscodingProgress {
	return model.TranscodingProgress{StreamID: stream, Version: v, Failed: true, Error: "The video could not be converted."}
}

func TestGetLectureCut(t *testing.T) {
	req := &protobuf.GetLectureCutRequest{CourseId: 1, StreamId: 7}

	t.Run("is not open without a cut", func(t *testing.T) {
		api, m := newCutAPI(t)
		m.streams.EXPECT().GetStreamByID(gomock.Any(), "7").Return(cuttableLecture(), nil)
		m.cuts.EXPECT().GetLectureCutBySource(uint(7)).Return(model.LectureCut{}, gorm.ErrRecordNotFound)
		resp, err := api.GetLectureCut(asCaller(lectureAdmin), req)
		if err != nil || resp.Open {
			t.Errorf("got %v, %v, want not open", resp, err)
		}
	})

	t.Run("lists the versions of the target", func(t *testing.T) {
		api, m := newCutAPI(t)
		m.streams.EXPECT().GetStreamByID(gomock.Any(), "7").Return(cuttableLecture(), nil)
		m.cuts.EXPECT().GetLectureCutBySource(uint(7)).Return(openDuplicate, nil)
		progressRows(m, 8, map[model.StreamVersion]model.TranscodingProgress{
			model.COMB: {StreamID: 8, Version: model.COMB, Progress: 40},
			model.CAM:  failedRow(8, model.CAM),
		})
		resp, err := api.GetLectureCut(asCaller(lectureAdmin), req)
		if err != nil {
			t.Fatal(err)
		}
		if !resp.Open || resp.TargetStreamId != 8 || !resp.Duplicate || len(resp.Progresses) != 2 ||
			resp.Progresses[0].Progress != 40 || !resp.Progresses[1].Failed || resp.Progresses[1].Version != "CAM" {
			t.Errorf("resp = %v", resp)
		}
	})

	t.Run("404s another course's lecture", func(t *testing.T) {
		api, m := newCutAPI(t)
		m.streams.EXPECT().GetStreamByID(gomock.Any(), "7").Return(foreignLecture(), nil)
		_, err := api.GetLectureCut(asCaller(lectureAdmin), req)
		wantCode(t, err, codes.NotFound)
	})
}

func TestRetryLectureCut(t *testing.T) {
	req := &protobuf.RetryLectureCutRequest{CourseId: 1, StreamId: 7}
	open := func(t *testing.T, cut model.LectureCut) (*API, cutMocks) {
		api, m := newCutAPI(t)
		m.streams.EXPECT().GetStreamByID(gomock.Any(), "7").Return(cuttableLecture(), nil)
		m.cuts.EXPECT().GetLectureCutBySource(uint(7)).Return(cut, nil)
		return api, m
	}

	t.Run("cuts only the versions that failed, with the ranges of the cut", func(t *testing.T) {
		api, m := open(t, openOverwrite)
		// The combined version is done and has no row any more.
		progressRows(m, 7, map[model.StreamVersion]model.TranscodingProgress{model.CAM: failedRow(7, model.CAM)})
		m.streams.EXPECT().SaveTranscodingProgress(model.TranscodingProgress{StreamID: 7, Version: model.CAM}).Return(nil)

		if _, err := api.RetryLectureCut(asCaller(lectureAdmin), req); err != nil {
			t.Fatal(err)
		}
		if len(m.cutting.cuts) != 1 {
			t.Fatalf("cuts = %+v, want the camera version only", m.cutting.cuts)
		}
		c := m.cutting.cuts[0]
		if c.src != (CutSource{model.CAM, "https://cam"}) || c.target != 7 || !reflect.DeepEqual(c.discard, openOverwrite.Discard) {
			t.Errorf("cut = %+v", c)
		}
	})

	t.Run("cuts the copy from the original", func(t *testing.T) {
		api, m := open(t, openDuplicate)
		progressRows(m, 8, map[model.StreamVersion]model.TranscodingProgress{
			model.COMB: failedRow(8, model.COMB), model.CAM: failedRow(8, model.CAM),
		})
		m.streams.EXPECT().SaveTranscodingProgress(gomock.Any()).Return(nil).Times(2)
		if _, err := api.RetryLectureCut(asCaller(lectureAdmin), req); err != nil {
			t.Fatal(err)
		}
		if len(m.cutting.cuts) != 2 || m.cutting.cuts[0].source != 7 || m.cutting.cuts[0].target != 8 {
			t.Errorf("cuts = %+v", m.cutting.cuts)
		}
	})

	t.Run("marks them failed again when no runner takes them", func(t *testing.T) {
		api, m := open(t, openOverwrite)
		m.cutting.cutErr = errors.New("no runner is available")
		progressRows(m, 7, map[model.StreamVersion]model.TranscodingProgress{model.CAM: failedRow(7, model.CAM)})
		var saved []model.TranscodingProgress
		m.streams.EXPECT().SaveTranscodingProgress(gomock.Any()).DoAndReturn(func(p model.TranscodingProgress) error {
			saved = append(saved, p)
			return nil
		}).Times(2)
		_, err := api.RetryLectureCut(asCaller(lectureAdmin), req)
		wantCode(t, err, codes.Unavailable)
		if len(saved) != 2 || saved[0].Failed || !saved[1].Failed {
			t.Errorf("rows = %+v, want one started and then marked failed", saved)
		}
	})

	t.Run("refuses while a version is being cut", func(t *testing.T) {
		api, m := open(t, openOverwrite)
		progressRows(m, 7, map[model.StreamVersion]model.TranscodingProgress{
			model.COMB: {StreamID: 7, Version: model.COMB, Progress: 30}, model.CAM: failedRow(7, model.CAM),
		})
		_, err := api.RetryLectureCut(asCaller(lectureAdmin), req)
		wantCode(t, err, codes.AlreadyExists)
		if len(m.cutting.cuts) != 0 {
			t.Error("a runner was asked")
		}
	})

	t.Run("refuses when no version failed", func(t *testing.T) {
		api, m := open(t, openOverwrite)
		progressRows(m, 7, nil)
		_, err := api.RetryLectureCut(asCaller(lectureAdmin), req)
		wantCode(t, err, codes.AlreadyExists)
	})

	t.Run("404s a lecture without an open cut", func(t *testing.T) {
		api, m := newCutAPI(t)
		m.streams.EXPECT().GetStreamByID(gomock.Any(), "7").Return(cuttableLecture(), nil)
		m.cuts.EXPECT().GetLectureCutBySource(uint(7)).Return(model.LectureCut{}, gorm.ErrRecordNotFound)
		_, err := api.RetryLectureCut(asCaller(lectureAdmin), req)
		wantCode(t, err, codes.NotFound)
	})

	t.Run("503s without runners", func(t *testing.T) {
		api, _ := open(t, openOverwrite)
		api.cutting = nil
		_, err := api.RetryLectureCut(asCaller(lectureAdmin), req)
		wantCode(t, err, codes.Unavailable)
	})
}

func TestAbandonLectureCut(t *testing.T) {
	req := &protobuf.AbandonLectureCutRequest{CourseId: 1, StreamId: 7}
	open := func(t *testing.T, cut model.LectureCut) (*API, cutMocks) {
		api, m := newCutAPI(t)
		m.streams.EXPECT().GetStreamByID(gomock.Any(), "7").Return(cuttableLecture(), nil)
		m.cuts.EXPECT().GetLectureCutBySource(uint(7)).Return(cut, nil)
		return api, m
	}

	t.Run("removes the copy of a duplicate", func(t *testing.T) {
		api, m := open(t, openDuplicate)
		progressRows(m, 8, map[model.StreamVersion]model.TranscodingProgress{model.CAM: failedRow(8, model.CAM)})
		m.streams.EXPECT().RemoveTranscodingProgress(model.CAM, uint(8)).Return(nil)
		m.streams.EXPECT().RemoveStream("8").Return(nil)
		m.cuts.EXPECT().DeleteLectureCut(uint(8)).Return(nil)

		resp, err := api.AbandonLectureCut(asCaller(lectureAdmin), req)
		if err != nil || len(resp.LeftUncut) != 0 {
			t.Errorf("got %v, %v, want the original untouched", resp, err)
		}
	})

	t.Run("keeps what is cut of a replaced recording and moves its data along", func(t *testing.T) {
		api, m := open(t, openOverwrite)
		progressRows(m, 7, map[model.StreamVersion]model.TranscodingProgress{model.CAM: failedRow(7, model.CAM)})
		m.cuts.EXPECT().FinishLectureCut(openOverwrite).Return(true, nil)
		m.streams.EXPECT().RemoveTranscodingProgress(model.CAM, uint(7)).Return(nil)

		resp, err := api.AbandonLectureCut(asCaller(lectureAdmin), req)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(resp.LeftUncut, []string{"CAM"}) {
			t.Errorf("left uncut = %v, want the camera version", resp.LeftUncut)
		}
		if !reflect.DeepEqual(m.cutting.scanned, []uint{7}) {
			t.Errorf("silences scanned for %v", m.cutting.scanned)
		}
	})

	t.Run("changes nothing when no version was cut", func(t *testing.T) {
		api, m := open(t, openOverwrite)
		progressRows(m, 7, map[model.StreamVersion]model.TranscodingProgress{
			model.COMB: failedRow(7, model.COMB), model.CAM: failedRow(7, model.CAM),
		})
		m.cuts.EXPECT().DeleteLectureCut(uint(7)).Return(nil)
		m.streams.EXPECT().RemoveTranscodingProgress(gomock.Any(), uint(7)).Return(nil).Times(2)

		resp, err := api.AbandonLectureCut(asCaller(lectureAdmin), req)
		if err != nil || len(resp.LeftUncut) != 0 {
			t.Errorf("got %v, %v, want nothing left uncut", resp, err)
		}
		if len(m.cutting.scanned) != 0 {
			t.Error("silences were scanned for a lecture that did not change")
		}
	})

	t.Run("leaves the data where it is when the combined version was not cut", func(t *testing.T) {
		api, m := open(t, openOverwrite)
		progressRows(m, 7, map[model.StreamVersion]model.TranscodingProgress{model.COMB: failedRow(7, model.COMB)})
		// no FinishLectureCut: the version viewers watch is as it was
		m.cuts.EXPECT().DeleteLectureCut(uint(7)).Return(nil)
		m.streams.EXPECT().RemoveTranscodingProgress(model.COMB, uint(7)).Return(nil)

		resp, err := api.AbandonLectureCut(asCaller(lectureAdmin), req)
		if err != nil || !reflect.DeepEqual(resp.LeftUncut, []string{"COMB"}) {
			t.Errorf("got %v, %v, want the combined version left uncut", resp, err)
		}
	})

	t.Run("keeps the cut open when it can't be finished", func(t *testing.T) {
		api, m := open(t, openOverwrite)
		progressRows(m, 7, map[model.StreamVersion]model.TranscodingProgress{model.CAM: failedRow(7, model.CAM)})
		m.cuts.EXPECT().FinishLectureCut(openOverwrite).Return(false, errors.New("db down"))
		// the failed rows stay too

		_, err := api.AbandonLectureCut(asCaller(lectureAdmin), req)
		wantCode(t, err, codes.Unknown)
	})

	t.Run("refuses while a version is being cut", func(t *testing.T) {
		api, m := open(t, openOverwrite)
		progressRows(m, 7, map[model.StreamVersion]model.TranscodingProgress{model.COMB: {StreamID: 7, Version: model.COMB, Progress: 5}})
		_, err := api.AbandonLectureCut(asCaller(lectureAdmin), req)
		wantCode(t, err, codes.AlreadyExists)
	})

	t.Run("404s a lecture without an open cut", func(t *testing.T) {
		api, m := newCutAPI(t)
		m.streams.EXPECT().GetStreamByID(gomock.Any(), "7").Return(cuttableLecture(), nil)
		m.cuts.EXPECT().GetLectureCutBySource(uint(7)).Return(model.LectureCut{}, gorm.ErrRecordNotFound)
		_, err := api.AbandonLectureCut(asCaller(lectureAdmin), req)
		wantCode(t, err, codes.NotFound)
	})
}
