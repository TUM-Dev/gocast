package runner_manager

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/tum-dev/gocast/runner/pkg/ptr"
	"github.com/tum-dev/gocast/runner/protobuf"
	"go.uber.org/mock/gomock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"

	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/mock_dao"
	"github.com/TUM-Dev/gocast/model"
)

// withoutCut gives w the DAOs a new VOD reaches, for a stream that has no cut under way.
func withoutCut(ctrl *gomock.Controller, w dao.DaoWrapper) dao.DaoWrapper {
	files := mock_dao.NewMockFileDao(ctrl)
	files.EXPECT().DeleteStreamFiles(gomock.Any(), model.FileType(model.FILETYPE_WAVEFORM)).Return(nil).AnyTimes()
	cuts := mock_dao.NewMockLectureCutDao(ctrl)
	cuts.EXPECT().GetLectureCut(gomock.Any()).Return(model.LectureCut{}, gorm.ErrRecordNotFound).AnyTimes()
	w.FileDao, w.LectureCutDao = files, cuts
	return w
}

func TestSaveWaveform(t *testing.T) {
	notification := func() *protobuf.WaveformReadyNotification {
		return &protobuf.WaveformReadyNotification{
			Stream:        &protobuf.StreamInfo{Id: ptr.Take(uint64(5))},
			Peaks:         []byte{0, 10, 255},
			BucketSeconds: ptr.Take(uint32(1)),
			Durations: []*protobuf.WaveformReadyNotification_VersionDuration{
				{StreamVersion: ptr.Take(protobuf.StreamVersion_STREAM_VERSION_COMBINED), Seconds: ptr.Take(100.5)},
				{StreamVersion: ptr.Take(protobuf.StreamVersion_STREAM_VERSION_CAMERA), Seconds: ptr.Take(99.0)},
			},
		}
	}

	t.Run("writes the file and records it", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		streamsDao := mock_dao.NewMockStreamsDao(ctrl)
		streamsDao.EXPECT().GetStreamByID(gomock.Any(), "5").Return(model.Stream{Model: gorm.Model{ID: 5}, CourseID: 3}, nil)
		var saved model.File
		fileDao := mock_dao.NewMockFileDao(ctrl)
		fileDao.EXPECT().SetStreamFile(uint(5), gomock.Any()).DoAndReturn(func(_ uint, f model.File) error { saved = f; return nil })

		storage := t.TempDir()
		m := New(dao.DaoWrapper{StreamsDao: streamsDao, FileDao: fileDao}, WithMassStorage(storage))
		if err := m.saveWaveform(context.Background(), notification()); err != nil {
			t.Fatal(err)
		}

		if want := filepath.Join(storage, "waveforms", "3", "5.json"); saved.Path != want || saved.Type != model.FILETYPE_WAVEFORM {
			t.Errorf("saved file = %+v, want %s of type waveform", saved, want)
		}
		b, err := os.ReadFile(saved.Path)
		if err != nil {
			t.Fatal(err)
		}
		var got model.Waveform
		if err := json.Unmarshal(b, &got); err != nil {
			t.Fatal(err)
		}
		if got.BucketSeconds != 1 || string(got.Peaks) != string([]byte{0, 10, 255}) ||
			got.Durations[model.COMB] != 100.5 || got.Durations[model.CAM] != 99 || len(got.Durations) != 2 {
			t.Errorf("waveform = %+v", got)
		}
	})

	t.Run("refuses an empty one", func(t *testing.T) {
		n := notification()
		n.Peaks = nil
		m := New(dao.DaoWrapper{})
		if err := m.saveWaveform(context.Background(), n); status.Code(err) != codes.InvalidArgument {
			t.Errorf("got %v, want InvalidArgument", err)
		}
	})

	t.Run("a missing stream is permanent", func(t *testing.T) {
		streamsDao := mock_dao.NewMockStreamsDao(gomock.NewController(t))
		streamsDao.EXPECT().GetStreamByID(gomock.Any(), "5").Return(model.Stream{}, gorm.ErrRecordNotFound)
		m := New(dao.DaoWrapper{StreamsDao: streamsDao})
		if err := m.saveWaveform(context.Background(), notification()); status.Code(err) != codes.NotFound {
			t.Errorf("got %v, want NotFound", err)
		}
	})
}

func TestAfterVODReady(t *testing.T) {
	cut := model.LectureCut{TargetStreamID: 5, SourceStreamID: 4, Discard: []model.CutRange{{Start: 1, End: 2}}}

	setup := func(t *testing.T) (*mock_dao.MockStreamsDao, *mock_dao.MockLectureCutDao, *Manager) {
		ctrl := gomock.NewController(t)
		files := mock_dao.NewMockFileDao(ctrl)
		files.EXPECT().DeleteStreamFiles(uint(5), model.FileType(model.FILETYPE_WAVEFORM)).Return(nil)
		streamsDao := mock_dao.NewMockStreamsDao(ctrl)
		cuts := mock_dao.NewMockLectureCutDao(ctrl)
		return streamsDao, cuts, New(dao.DaoWrapper{StreamsDao: streamsDao, FileDao: files, LectureCutDao: cuts})
	}

	t.Run("no cut under way", func(t *testing.T) {
		_, cuts, m := setup(t)
		cuts.EXPECT().GetLectureCut(uint(5)).Return(model.LectureCut{}, gorm.ErrRecordNotFound)
		if err := m.afterVODReady(context.Background(), 5); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("another version is still converting", func(t *testing.T) {
		streamsDao, cuts, m := setup(t)
		cuts.EXPECT().GetLectureCut(uint(5)).Return(cut, nil)
		streamsDao.EXPECT().GetTranscodingProgressByVersion(model.COMB, uint(5)).Return(model.TranscodingProgress{}, gorm.ErrRecordNotFound)
		streamsDao.EXPECT().GetTranscodingProgressByVersion(model.CAM, uint(5)).Return(model.TranscodingProgress{Progress: 40}, nil)
		// FinishLectureCut is not expected
		if err := m.afterVODReady(context.Background(), 5); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("a failed version keeps the cut open", func(t *testing.T) {
		streamsDao, cuts, m := setup(t)
		cuts.EXPECT().GetLectureCut(uint(5)).Return(cut, nil)
		streamsDao.EXPECT().GetTranscodingProgressByVersion(model.COMB, uint(5)).Return(model.TranscodingProgress{Failed: true}, nil)
		if err := m.afterVODReady(context.Background(), 5); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("the last version finishes it", func(t *testing.T) {
		streamsDao, cuts, m := setup(t)
		cuts.EXPECT().GetLectureCut(uint(5)).Return(cut, nil)
		streamsDao.EXPECT().GetTranscodingProgressByVersion(gomock.Any(), uint(5)).Return(model.TranscodingProgress{}, gorm.ErrRecordNotFound).Times(3)
		cuts.EXPECT().FinishLectureCut(cut).Return(true, nil)
		// The silence scan that follows can't be asked for here, which is only logged.
		streamsDao.EXPECT().GetStreamByID(gomock.Any(), "5").Return(model.Stream{}, errors.New("db down"))
		if err := m.afterVODReady(context.Background(), 5); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("a cut another version finished meanwhile is left alone", func(t *testing.T) {
		streamsDao, cuts, m := setup(t)
		cuts.EXPECT().GetLectureCut(uint(5)).Return(cut, nil)
		streamsDao.EXPECT().GetTranscodingProgressByVersion(gomock.Any(), uint(5)).Return(model.TranscodingProgress{}, gorm.ErrRecordNotFound).Times(3)
		cuts.EXPECT().FinishLectureCut(cut).Return(false, nil)
		// no silence scan: GetStreamByID is not expected
		if err := m.afterVODReady(context.Background(), 5); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("a cut that can't be finished is retried by the runner", func(t *testing.T) {
		streamsDao, cuts, m := setup(t)
		cuts.EXPECT().GetLectureCut(uint(5)).Return(cut, nil)
		streamsDao.EXPECT().GetTranscodingProgressByVersion(gomock.Any(), uint(5)).Return(model.TranscodingProgress{}, gorm.ErrRecordNotFound).Times(3)
		cuts.EXPECT().FinishLectureCut(cut).Return(false, errors.New("db down"))
		if err := m.afterVODReady(context.Background(), 5); status.Code(err) != codes.Internal {
			t.Errorf("got %v, want Internal", err)
		}
	})
}
