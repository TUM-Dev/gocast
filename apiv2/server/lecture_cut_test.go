package apiv2

import (
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
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
	cutErr          error
	cutSource       uint
	cutTarget       uint
	cutSources      []CutSource
	cutDiscard      []model.CutRange
}

func (f *fakeCutting) RequestWaveform(streamID uint, sources []CutSource) error {
	f.waveformFor = append(f.waveformFor, streamID)
	f.waveformSources = sources
	return f.cutErr
}

func (f *fakeCutting) Cut(source, target uint, sources []CutSource, discard []model.CutRange) error {
	f.cutSource, f.cutTarget, f.cutSources, f.cutDiscard = source, target, sources, discard
	return f.cutErr
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
		m.cutting.cutErr = errors.New("no runner is available")
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
			if err == nil && len(got) != len(tc.in) {
				t.Errorf("got %v", got)
			}
			if err != nil {
				wantCode(t, err, codes.InvalidArgument)
			}
		})
	}
}

func TestCutLecture(t *testing.T) {
	req := func(mode protobuf.LectureCutMode, d ...float64) *protobuf.CutLectureRequest {
		return &protobuf.CutLectureRequest{CourseId: 1, StreamId: 7, Mode: mode, Discard: discard(d...)}
	}
	overwrite := protobuf.LectureCutMode_LECTURE_CUT_MODE_OVERWRITE
	duplicate := protobuf.LectureCutMode_LECTURE_CUT_MODE_DUPLICATE
	idle := func(m cutMocks) {
		m.streams.EXPECT().GetTranscodingProgressByVersion(gomock.Any(), uint(7)).Return(model.TranscodingProgress{}, gorm.ErrRecordNotFound).Times(3)
	}

	t.Run("overwrites in place", func(t *testing.T) {
		api, m := newCutAPI(t)
		m.streams.EXPECT().GetStreamByID(gomock.Any(), "7").Return(cuttableLecture(), nil)
		idle(m)
		storeWaveform(t, m, hourLong)
		want := model.LectureCut{TargetStreamID: 7, SourceStreamID: 7, Discard: []model.CutRange{{Start: 60, End: 120}, {Start: 300, End: 400}}}
		m.cuts.EXPECT().SaveLectureCut(want).Return(nil)
		for _, v := range []model.StreamVersion{model.COMB, model.CAM} {
			m.streams.EXPECT().SaveTranscodingProgress(model.TranscodingProgress{StreamID: 7, Version: v}).Return(nil)
		}

		resp, err := api.CutLecture(asCaller(lectureAdmin), req(overwrite, 60, 120, 300, 400))
		if err != nil {
			t.Fatal(err)
		}
		if resp.StreamId != 7 || m.cutting.cutSource != 7 || m.cutting.cutTarget != 7 {
			t.Errorf("resp = %v, cut %d -> %d", resp, m.cutting.cutSource, m.cutting.cutTarget)
		}
		wantSources := []CutSource{{model.COMB, "https://comb?jwt=signed"}, {model.CAM, "https://cam"}}
		if !reflect.DeepEqual(m.cutting.cutSources, wantSources) {
			t.Errorf("sources = %v, want %v", m.cutting.cutSources, wantSources)
		}
		if len(m.cutting.cutDiscard) != 2 || m.cutting.cutDiscard[1].End != 400 {
			t.Errorf("discard = %v", m.cutting.cutDiscard)
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
		m.cuts.EXPECT().SaveLectureCut(model.LectureCut{
			TargetStreamID: 8, SourceStreamID: 7, SourcePrivate: false, Discard: []model.CutRange{{Start: 60, End: 120}},
		}).Return(nil)
		for _, v := range []model.StreamVersion{model.COMB, model.CAM} {
			m.streams.EXPECT().SaveTranscodingProgress(model.TranscodingProgress{StreamID: 8, Version: v}).Return(nil)
		}

		resp, err := api.CutLecture(asCaller(lectureAdmin), req(duplicate, 60, 120))
		if err != nil {
			t.Fatal(err)
		}
		if resp.StreamId != 8 || m.cutting.cutSource != 7 || m.cutting.cutTarget != 8 {
			t.Errorf("resp = %v, cut %d -> %d", resp, m.cutting.cutSource, m.cutting.cutTarget)
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
		m.cuts.EXPECT().SaveLectureCut(gomock.Any()).DoAndReturn(func(c model.LectureCut) error {
			if !c.SourcePrivate {
				t.Error("SourcePrivate not set")
			}
			return nil
		})
		m.streams.EXPECT().SaveTranscodingProgress(gomock.Any()).Return(nil).Times(2)
		if _, err := api.CutLecture(asCaller(lectureAdmin), req(duplicate, 60, 120)); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("takes back a duplicate that no runner took", func(t *testing.T) {
		api, m := newCutAPI(t)
		m.cutting.cutErr = errors.New("no runner is available")
		m.streams.EXPECT().GetStreamByID(gomock.Any(), "7").Return(cuttableLecture(), nil)
		idle(m)
		storeWaveform(t, m, hourLong)
		m.streams.EXPECT().CreateStream(gomock.Any()).DoAndReturn(func(s *model.Stream) error { s.ID = 8; return nil })
		m.cuts.EXPECT().SaveLectureCut(gomock.Any()).Return(nil)
		m.streams.EXPECT().SaveTranscodingProgress(gomock.Any()).Return(nil).Times(2)
		m.cuts.EXPECT().DeleteLectureCut(uint(8)).Return(nil)
		m.streams.EXPECT().RemoveTranscodingProgress(gomock.Any(), uint(8)).Return(nil).Times(2)
		m.streams.EXPECT().RemoveStream("8").Return(nil)

		_, err := api.CutLecture(asCaller(lectureAdmin), req(duplicate, 60, 120))
		wantCode(t, err, codes.Unavailable)
	})

	t.Run("does not cut without the markers that tell when it is done", func(t *testing.T) {
		api, m := newCutAPI(t)
		m.streams.EXPECT().GetStreamByID(gomock.Any(), "7").Return(cuttableLecture(), nil)
		idle(m)
		storeWaveform(t, m, hourLong)
		m.cuts.EXPECT().SaveLectureCut(gomock.Any()).Return(nil)
		m.streams.EXPECT().SaveTranscodingProgress(gomock.Any()).Return(errors.New("db down"))
		m.cuts.EXPECT().DeleteLectureCut(uint(7)).Return(nil)
		m.streams.EXPECT().RemoveTranscodingProgress(gomock.Any(), uint(7)).Return(nil).Times(2)

		_, err := api.CutLecture(asCaller(lectureAdmin), req(overwrite, 60, 120))
		wantCode(t, err, codes.Unknown)
		if m.cutting.cutTarget != 0 {
			t.Error("a runner was asked")
		}
	})

	t.Run("marks an overwrite that no runner took as failed", func(t *testing.T) {
		api, m := newCutAPI(t)
		m.cutting.cutErr = errors.New("no runner is available")
		m.streams.EXPECT().GetStreamByID(gomock.Any(), "7").Return(cuttableLecture(), nil)
		idle(m)
		storeWaveform(t, m, hourLong)
		m.cuts.EXPECT().SaveLectureCut(gomock.Any()).Return(nil)
		m.streams.EXPECT().SaveTranscodingProgress(gomock.Any()).DoAndReturn(func(p model.TranscodingProgress) error { return nil }).Times(4)
		m.cuts.EXPECT().DeleteLectureCut(uint(7)).Return(nil)

		_, err := api.CutLecture(asCaller(lectureAdmin), req(overwrite, 60, 120))
		wantCode(t, err, codes.Unavailable)
	})

	t.Run("refuses while a version is converting", func(t *testing.T) {
		api, m := newCutAPI(t)
		m.streams.EXPECT().GetStreamByID(gomock.Any(), "7").Return(cuttableLecture(), nil)
		m.streams.EXPECT().GetTranscodingProgressByVersion(model.COMB, uint(7)).Return(model.TranscodingProgress{Progress: 10}, nil)
		_, err := api.CutLecture(asCaller(lectureAdmin), req(overwrite, 60, 120))
		wantCode(t, err, codes.AlreadyExists)
	})

	t.Run("cuts again after a failure", func(t *testing.T) {
		api, m := newCutAPI(t)
		m.streams.EXPECT().GetStreamByID(gomock.Any(), "7").Return(cuttableLecture(), nil)
		m.streams.EXPECT().GetTranscodingProgressByVersion(model.COMB, uint(7)).Return(model.TranscodingProgress{Failed: true}, nil)
		m.streams.EXPECT().GetTranscodingProgressByVersion(model.CAM, uint(7)).Return(model.TranscodingProgress{}, gorm.ErrRecordNotFound)
		m.streams.EXPECT().GetTranscodingProgressByVersion(model.PRES, uint(7)).Return(model.TranscodingProgress{}, gorm.ErrRecordNotFound)
		storeWaveform(t, m, hourLong)
		m.cuts.EXPECT().SaveLectureCut(gomock.Any()).Return(nil)
		m.streams.EXPECT().SaveTranscodingProgress(gomock.Any()).Return(nil).Times(2)
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
		if m.cutting.cutTarget != 0 {
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
		m.cuts.EXPECT().SaveLectureCut(gomock.Any()).Return(nil)
		m.streams.EXPECT().SaveTranscodingProgress(gomock.Any()).Return(nil).Times(2)
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
