package runner_manager

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go.uber.org/mock/gomock"
	"gorm.io/gorm"

	"github.com/tum-dev/gocast/runner/pkg/ptr"
	"github.com/tum-dev/gocast/runner/protobuf"

	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/mock_dao"
	"github.com/TUM-Dev/gocast/model"
)

// TestSaveSectionImages checks that the images a runner sends are written below mass
// storage at a path built from ids only, and that the video sections are pointed at them.
func TestSaveSectionImages(t *testing.T) {
	ctrl := gomock.NewController(t)
	mass := t.TempDir()

	stream := model.Stream{
		Model:    gorm.Model{ID: 1024},
		CourseID: 500,
		Start:    time.Date(2025, 10, 2, 10, 0, 0, 0, time.UTC),
	}

	streamsDao := mock_dao.NewMockStreamsDao(ctrl)
	streamsDao.EXPECT().GetStreamByID(gomock.Any(), "1024").Return(stream, nil)

	var written []model.File
	fileDao := mock_dao.NewMockFileDao(ctrl)
	fileDao.EXPECT().NewFile(gomock.Any()).Times(2).DoAndReturn(func(f *model.File) error {
		f.Model = gorm.Model{ID: uint(len(written) + 7)}
		written = append(written, *f)
		return nil
	})

	var updated []model.VideoSection
	sectionDao := mock_dao.NewMockVideoSectionDao(ctrl)
	sectionDao.EXPECT().Get(gomock.Any()).Times(2).Return(model.VideoSection{StreamID: 1024}, nil)
	sectionDao.EXPECT().Update(gomock.Any()).Times(2).DoAndReturn(func(s *model.VideoSection) error {
		updated = append(updated, *s)
		return nil
	})

	m := New(dao.DaoWrapper{StreamsDao: streamsDao, FileDao: fileDao, VideoSectionDao: sectionDao},
		WithMassStorage(mass))

	err := m.saveSectionImages(context.Background(), &protobuf.SectionImagesReadyNotification{
		Stream: &protobuf.StreamInfo{Id: ptr.Take(uint64(1024))},
		Images: []*protobuf.SectionImage{
			{SectionId: ptr.Take(uint64(42)), Image: []byte("first")},
			{SectionId: ptr.Take(uint64(43)), Image: []byte("second")},
		},
	})
	if err != nil {
		t.Fatalf("saveSectionImages: %v", err)
	}

	wantDir := filepath.Join(mass, "sections", "2025/10", "500")
	for i, want := range []struct {
		name    string
		content string
	}{
		{"1024_42.jpg", "first"},
		{"1024_43.jpg", "second"},
	} {
		path := filepath.Join(wantDir, want.name)
		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("reading %s: %v", path, err)
		}
		if string(got) != want.content {
			t.Errorf("%s = %q, want %q", path, got, want.content)
		}
		if written[i].Path != path {
			t.Errorf("file %d stored at %q, want %q", i, written[i].Path, path)
		}
		if written[i].Type != model.FILETYPE_IMAGE_JPG {
			t.Errorf("file %d has type %v, want %v", i, written[i].Type, model.FILETYPE_IMAGE_JPG)
		}
		if written[i].StreamID != stream.ID {
			t.Errorf("file %d has stream %d, want %d", i, written[i].StreamID, stream.ID)
		}
	}

	if len(updated) != 2 || updated[0].ID != 42 || updated[1].ID != 43 {
		t.Fatalf("updated sections = %+v, want ids 42 and 43", updated)
	}
	if updated[0].FileID != written[0].ID || updated[1].FileID != written[1].ID {
		t.Errorf("sections point at the wrong files: %+v vs %+v", updated, written)
	}
}

// TestSaveSectionImagesStaysInMassStorage is the regression check for the traversal this
// replaces: the runner has no say in the path, so nothing it sends can leave mass storage.
func TestSaveSectionImagesStaysInMassStorage(t *testing.T) {
	ctrl := gomock.NewController(t)
	mass := t.TempDir()

	streamsDao := mock_dao.NewMockStreamsDao(ctrl)
	streamsDao.EXPECT().GetStreamByID(gomock.Any(), "1").Return(model.Stream{
		Model:    gorm.Model{ID: 1},
		CourseID: 2,
		Start:    time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
	}, nil)

	var stored string
	fileDao := mock_dao.NewMockFileDao(ctrl)
	fileDao.EXPECT().NewFile(gomock.Any()).DoAndReturn(func(f *model.File) error {
		stored = f.Path
		return nil
	})

	sectionDao := mock_dao.NewMockVideoSectionDao(ctrl)
	sectionDao.EXPECT().Get(gomock.Any()).Return(model.VideoSection{StreamID: 1}, nil)
	sectionDao.EXPECT().Update(gomock.Any()).Return(nil)

	m := New(dao.DaoWrapper{StreamsDao: streamsDao, FileDao: fileDao, VideoSectionDao: sectionDao},
		WithMassStorage(mass))

	err := m.saveSectionImages(context.Background(), &protobuf.SectionImagesReadyNotification{
		Stream: &protobuf.StreamInfo{Id: ptr.Take(uint64(1))},
		Images: []*protobuf.SectionImage{{SectionId: ptr.Take(uint64(9)), Image: []byte("x")}},
	})
	if err != nil {
		t.Fatalf("saveSectionImages: %v", err)
	}

	rel, err := filepath.Rel(mass, stored)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		t.Fatalf("section image stored at %s, which is outside of %s", stored, mass)
	}
}

// TestSaveSectionImagesReplacesPreviousFile checks that regenerating a section image
// drops the file row the section used to point at instead of orphaning it.
func TestSaveSectionImagesReplacesPreviousFile(t *testing.T) {
	ctrl := gomock.NewController(t)
	mass := t.TempDir()

	streamsDao := mock_dao.NewMockStreamsDao(ctrl)
	streamsDao.EXPECT().GetStreamByID(gomock.Any(), "1").Return(model.Stream{
		Model:    gorm.Model{ID: 1},
		CourseID: 2,
		Start:    time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
	}, nil)

	fileDao := mock_dao.NewMockFileDao(ctrl)
	fileDao.EXPECT().NewFile(gomock.Any()).DoAndReturn(func(f *model.File) error {
		f.Model = gorm.Model{ID: 99}
		return nil
	})
	// the row the section pointed at before is the one that has to go
	fileDao.EXPECT().DeleteFile(uint(31)).Return(nil)

	sectionDao := mock_dao.NewMockVideoSectionDao(ctrl)
	sectionDao.EXPECT().Get(uint(9)).Return(model.VideoSection{StreamID: 1, FileID: 31}, nil)
	sectionDao.EXPECT().Update(gomock.Any()).DoAndReturn(func(s *model.VideoSection) error {
		if s.FileID != 99 {
			t.Errorf("section points at file %d, want the new file 99", s.FileID)
		}
		return nil
	})

	m := New(dao.DaoWrapper{StreamsDao: streamsDao, FileDao: fileDao, VideoSectionDao: sectionDao},
		WithMassStorage(mass))

	err := m.saveSectionImages(context.Background(), &protobuf.SectionImagesReadyNotification{
		Stream: &protobuf.StreamInfo{Id: ptr.Take(uint64(1))},
		Images: []*protobuf.SectionImage{{SectionId: ptr.Take(uint64(9)), Image: []byte("x")}},
	})
	if err != nil {
		t.Fatalf("saveSectionImages: %v", err)
	}
}

// TestRequestSectionImagesNoSections makes sure an empty request never reserves a runner.
func TestRequestSectionImagesNoSections(t *testing.T) {
	m := New(dao.DaoWrapper{})
	if err := m.RequestSectionImages(context.Background(), SectionImageRequest{StreamID: 1}); err != nil {
		t.Errorf("expected no error for an empty request, got %v", err)
	}
}

// TestGenerateSectionImagesFallback checks that the worker path is only used when no
// runner manager is available, and never for an empty request.
func TestGenerateSectionImagesFallback(t *testing.T) {
	sections := []model.VideoSection{{Model: gorm.Model{ID: 1}, StreamID: 1}}

	t.Run("no manager uses the fallback", func(t *testing.T) {
		called := false
		err := GenerateSectionImages(nil, SectionImageRequest{StreamID: 1, PlaylistURL: "x", Sections: sections},
			func() error { called = true; return nil })
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !called {
			t.Error("fallback was not called without a runner manager")
		}
	})

	t.Run("empty request does nothing", func(t *testing.T) {
		err := GenerateSectionImages(nil, SectionImageRequest{StreamID: 1},
			func() error { t.Error("fallback called for an empty request"); return nil })
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

// TestSaveSectionImagesDeletedStream checks that images for a stream deleted in the
// meantime are dropped rather than failing: the runner retries every error it gets
// back, so an error here would be retried forever.
func TestSaveSectionImagesDeletedStream(t *testing.T) {
	ctrl := gomock.NewController(t)
	mass := t.TempDir()

	streamsDao := mock_dao.NewMockStreamsDao(ctrl)
	streamsDao.EXPECT().GetStreamByID(gomock.Any(), "1").Return(model.Stream{}, gorm.ErrRecordNotFound)

	// no FileDao or VideoSectionDao expectations: nothing may be written
	m := New(dao.DaoWrapper{
		StreamsDao:      streamsDao,
		FileDao:         mock_dao.NewMockFileDao(ctrl),
		VideoSectionDao: mock_dao.NewMockVideoSectionDao(ctrl),
	}, WithMassStorage(mass))

	err := m.saveSectionImages(context.Background(), &protobuf.SectionImagesReadyNotification{
		Stream: &protobuf.StreamInfo{Id: ptr.Take(uint64(1))},
		Images: []*protobuf.SectionImage{{SectionId: ptr.Take(uint64(9)), Image: []byte("x")}},
	})
	if err != nil {
		t.Fatalf("saveSectionImages for a deleted stream = %v, want nil", err)
	}
	if entries, _ := os.ReadDir(mass); len(entries) != 0 {
		t.Errorf("mass storage is not empty: %v", entries)
	}
}

// TestSaveSectionImagesDeletedSection checks that the image of a section deleted in the
// meantime is dropped, while the images of the remaining sections are still saved.
func TestSaveSectionImagesDeletedSection(t *testing.T) {
	ctrl := gomock.NewController(t)
	mass := t.TempDir()

	streamsDao := mock_dao.NewMockStreamsDao(ctrl)
	streamsDao.EXPECT().GetStreamByID(gomock.Any(), "1").Return(model.Stream{
		Model:    gorm.Model{ID: 1},
		CourseID: 2,
		Start:    time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
	}, nil)

	fileDao := mock_dao.NewMockFileDao(ctrl)
	fileDao.EXPECT().NewFile(gomock.Any()).Times(1).Return(nil)

	sectionDao := mock_dao.NewMockVideoSectionDao(ctrl)
	sectionDao.EXPECT().Get(uint(8)).Return(model.VideoSection{}, gorm.ErrRecordNotFound)
	sectionDao.EXPECT().Get(uint(9)).Return(model.VideoSection{StreamID: 1}, nil)
	sectionDao.EXPECT().Update(gomock.Any()).Times(1).DoAndReturn(func(s *model.VideoSection) error {
		if s.ID != 9 {
			t.Errorf("updated section %d, want only the remaining section 9", s.ID)
		}
		return nil
	})

	m := New(dao.DaoWrapper{StreamsDao: streamsDao, FileDao: fileDao, VideoSectionDao: sectionDao},
		WithMassStorage(mass))

	err := m.saveSectionImages(context.Background(), &protobuf.SectionImagesReadyNotification{
		Stream: &protobuf.StreamInfo{Id: ptr.Take(uint64(1))},
		Images: []*protobuf.SectionImage{
			{SectionId: ptr.Take(uint64(8)), Image: []byte("deleted")},
			{SectionId: ptr.Take(uint64(9)), Image: []byte("kept")},
		},
	})
	if err != nil {
		t.Fatalf("saveSectionImages: %v", err)
	}

	dir := filepath.Join(mass, "sections", "2025/01", "2")
	if _, err := os.Stat(filepath.Join(dir, "1_8.jpg")); !os.IsNotExist(err) {
		t.Errorf("image of the deleted section was written (stat err: %v)", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "1_9.jpg")); err != nil {
		t.Errorf("image of the remaining section is missing: %v", err)
	}
}

// TestSaveSectionImagesSectionOfAnotherStream checks that a section id belonging to a
// different stream is not repointed at the image.
func TestSaveSectionImagesSectionOfAnotherStream(t *testing.T) {
	ctrl := gomock.NewController(t)
	mass := t.TempDir()

	streamsDao := mock_dao.NewMockStreamsDao(ctrl)
	streamsDao.EXPECT().GetStreamByID(gomock.Any(), "1").Return(model.Stream{
		Model:    gorm.Model{ID: 1},
		CourseID: 2,
		Start:    time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
	}, nil)

	sectionDao := mock_dao.NewMockVideoSectionDao(ctrl)
	sectionDao.EXPECT().Get(uint(9)).Return(model.VideoSection{Model: gorm.Model{ID: 9}, StreamID: 77}, nil)

	// no NewFile or Update expectations: the foreign section must be left alone
	m := New(dao.DaoWrapper{
		StreamsDao:      streamsDao,
		FileDao:         mock_dao.NewMockFileDao(ctrl),
		VideoSectionDao: sectionDao,
	}, WithMassStorage(mass))

	err := m.saveSectionImages(context.Background(), &protobuf.SectionImagesReadyNotification{
		Stream: &protobuf.StreamInfo{Id: ptr.Take(uint64(1))},
		Images: []*protobuf.SectionImage{{SectionId: ptr.Take(uint64(9)), Image: []byte("x")}},
	})
	if err != nil {
		t.Fatalf("saveSectionImages: %v", err)
	}
	if _, err := os.Stat(filepath.Join(mass, "sections", "2025/01", "2", "1_9.jpg")); !os.IsNotExist(err) {
		t.Errorf("image for a section of another stream was written (stat err: %v)", err)
	}
}
