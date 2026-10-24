package runner_manager

import (
	"context"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
	"testing"
	"time"

	"go.uber.org/mock/gomock"
	"gorm.io/gorm"

	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/mock_dao"
	"github.com/TUM-Dev/gocast/model"
	"github.com/tum-dev/gocast/runner/pkg/ptr"
	"github.com/tum-dev/gocast/runner/protobuf"
)

func writeSolidJPEG(t *testing.T, path string, w, h int, c color.Color) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, c)
		}
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := jpeg.Encode(f, img, &jpeg.Options{Quality: 100}); err != nil {
		t.Fatal(err)
	}
}

func isNear(got color.Color, want color.RGBA) bool {
	r, g, b, _ := got.RGBA()
	d := func(a uint32, b uint8) bool {
		v := int(a>>8) - int(b)
		return v > -24 && v < 24
	}
	return d(r, want.R) && d(g, want.G) && d(b, want.B)
}

func TestCombineThumbnails(t *testing.T) {
	dir := t.TempDir()
	red, blue := color.RGBA{R: 255, A: 255}, color.RGBA{B: 255, A: 255}
	pres, cam, dest := filepath.Join(dir, "pres.jpeg"), filepath.Join(dir, "cam.jpeg"), filepath.Join(dir, "out.jpeg")
	writeSolidJPEG(t, pres, 720, 405, red)
	writeSolidJPEG(t, cam, 720, 405, blue)

	if err := combineThumbnails(pres, cam, dest); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(dest)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	out, err := jpeg.Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	if b := out.Bounds(); b.Dx() != 720 || b.Dy() != 405 {
		t.Errorf("size = %v, want the presentation's 720x405", b)
	}
	if c := out.At(100, 300); !isNear(c, red) {
		t.Errorf("bottom left = %v, want the presentation", c)
	}
	if c := out.At(720-60, 5+30); !isNear(c, blue) {
		t.Errorf("top right = %v, want the camera inset", c)
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 3 {
		t.Errorf("%d files left in the directory, want 3 (no temporary file)", len(entries))
	}
}

func TestComposeCamPres(t *testing.T) {
	dir := t.TempDir()
	pres, cam := filepath.Join(dir, "pres.jpeg"), filepath.Join(dir, "cam.jpeg")
	writeSolidJPEG(t, pres, 90, 60, color.RGBA{R: 255, A: 255})
	writeSolidJPEG(t, cam, 90, 60, color.RGBA{B: 255, A: 255})

	t.Run("both present: saves the composed file", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		files := mock_dao.NewMockFileDao(ctrl)
		files.EXPECT().GetThumbnail(uint(7), model.FileType(model.FILETYPE_THUMB_LG_PRES)).Return(model.File{Path: pres}, nil)
		files.EXPECT().GetThumbnail(uint(7), model.FileType(model.FILETYPE_THUMB_LG_CAM)).Return(model.File{Path: cam}, nil)
		var saved model.File
		files.EXPECT().SetThumbnail(uint(7), gomock.Any()).DoAndReturn(func(_ uint, f model.File) error { saved = f; return nil })

		New(dao.DaoWrapper{FileDao: files}).composeCamPres(7, dir)
		if saved.Type != model.FILETYPE_THUMB_LG_CAM_PRES || saved.Path != filepath.Join(dir, "7_CAM_PRES.jpeg") {
			t.Errorf("saved %+v", saved)
		}
		if _, err := os.Stat(saved.Path); err != nil {
			t.Error(err)
		}
	})

	t.Run("one missing: nothing is composed or saved", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		files := mock_dao.NewMockFileDao(ctrl)
		files.EXPECT().GetThumbnail(uint(8), model.FileType(model.FILETYPE_THUMB_LG_PRES)).Return(model.File{Path: pres}, nil)
		files.EXPECT().GetThumbnail(uint(8), model.FileType(model.FILETYPE_THUMB_LG_CAM)).Return(model.File{}, gorm.ErrRecordNotFound)

		New(dao.DaoWrapper{FileDao: files}).composeCamPres(8, dir) // no SetThumbnail expected
	})

	t.Run("unreadable source: the stale composite is dropped", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		files := mock_dao.NewMockFileDao(ctrl)
		files.EXPECT().GetThumbnail(uint(9), model.FileType(model.FILETYPE_THUMB_LG_PRES)).Return(model.File{Path: filepath.Join(dir, "gone.jpeg")}, nil)
		files.EXPECT().GetThumbnail(uint(9), model.FileType(model.FILETYPE_THUMB_LG_CAM)).Return(model.File{Path: cam}, nil)
		files.EXPECT().GetThumbnail(uint(9), model.FileType(model.FILETYPE_THUMB_LG_CAM_PRES)).Return(model.File{Model: gorm.Model{ID: 33}}, nil)
		files.EXPECT().DeleteFile(uint(33)).Return(nil) // GetLGThumbnail would keep preferring it

		New(dao.DaoWrapper{FileDao: files}).composeCamPres(9, dir) // no SetThumbnail expected
	})

	t.Run("unreadable source and no stale composite: nothing to drop", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		files := mock_dao.NewMockFileDao(ctrl)
		files.EXPECT().GetThumbnail(uint(9), model.FileType(model.FILETYPE_THUMB_LG_PRES)).Return(model.File{Path: filepath.Join(dir, "gone.jpeg")}, nil)
		files.EXPECT().GetThumbnail(uint(9), model.FileType(model.FILETYPE_THUMB_LG_CAM)).Return(model.File{Path: cam}, nil)
		files.EXPECT().GetThumbnail(uint(9), model.FileType(model.FILETYPE_THUMB_LG_CAM_PRES)).Return(model.File{}, gorm.ErrRecordNotFound)

		New(dao.DaoWrapper{FileDao: files}).composeCamPres(9, dir) // no DeleteFile expected
	})

	t.Run("lookup failure: nothing is composed", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		files := mock_dao.NewMockFileDao(ctrl)
		files.EXPECT().GetThumbnail(uint(10), model.FileType(model.FILETYPE_THUMB_LG_PRES)).Return(model.File{}, errors.New("driver: bad connection"))

		New(dao.DaoWrapper{FileDao: files}).composeCamPres(10, dir) // the CAM is not even asked for
	})
}

func saveThumbnailRequest(version protobuf.StreamVersion, data []byte) *protobuf.ThumbnailReadyNotification {
	return &protobuf.ThumbnailReadyNotification{
		Stream:        &protobuf.StreamInfo{Id: ptr.Take(uint64(5))},
		StreamVersion: &version,
		Thumbnail:     data,
	}
}

func TestSaveThumbnailComposes(t *testing.T) {
	stream := model.Stream{Model: gorm.Model{ID: 5}, CourseID: 2, Start: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)}
	newManager := func(t *testing.T, files *mock_dao.MockFileDao, streams *mock_dao.MockStreamsDao) *Manager {
		m := New(dao.DaoWrapper{FileDao: files, StreamsDao: streams})
		m.massStorage = t.TempDir()
		return m
	}

	for name, tc := range map[string]struct {
		version protobuf.StreamVersion
		compose bool
	}{
		"camera":       {protobuf.StreamVersion_STREAM_VERSION_CAMERA, true},
		"presentation": {protobuf.StreamVersion_STREAM_VERSION_PRESENTATION, true},
		"combined":     {protobuf.StreamVersion_STREAM_VERSION_COMBINED, false},
	} {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			streams := mock_dao.NewMockStreamsDao(ctrl)
			files := mock_dao.NewMockFileDao(ctrl)
			streams.EXPECT().GetStreamByID(gomock.Any(), "5").Return(stream, nil)
			files.EXPECT().SetThumbnail(uint(5), gomock.Any()).Return(nil)
			if tc.compose {
				// the other thumbnail is not there yet, which ends the composition
				files.EXPECT().GetThumbnail(uint(5), gomock.Any()).Return(model.File{}, gorm.ErrRecordNotFound)
			}
			// a COMB thumbnail asking the FileDao anything else would fail as an unexpected call
			if err := newManager(t, files, streams).saveThumbnail(context.Background(), saveThumbnailRequest(tc.version, []byte("jpeg"))); err != nil {
				t.Fatal(err)
			}
		})
	}
}

// A regenerated thumbnail can be smaller than the one before. It has to replace it whole
// and leave no temporary file behind.
func TestSaveThumbnailReplacesTheFile(t *testing.T) {
	stream := model.Stream{Model: gorm.Model{ID: 5}, CourseID: 2, Start: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)}
	ctrl := gomock.NewController(t)
	streams := mock_dao.NewMockStreamsDao(ctrl)
	files := mock_dao.NewMockFileDao(ctrl)
	streams.EXPECT().GetStreamByID(gomock.Any(), "5").Return(stream, nil).Times(2)
	files.EXPECT().SetThumbnail(uint(5), gomock.Any()).Return(nil).Times(2)

	m := New(dao.DaoWrapper{FileDao: files, StreamsDao: streams})
	m.massStorage = t.TempDir()
	var path string
	for _, data := range []string{"a much longer first thumbnail", "short"} {
		if err := m.saveThumbnail(context.Background(), saveThumbnailRequest(protobuf.StreamVersion_STREAM_VERSION_COMBINED, []byte(data))); err != nil {
			t.Fatal(err)
		}
		path = filepath.Join(m.massStorage, "thumbs", "2026/10", "2", "5_STREAM_VERSION_COMBINED.jpeg")
		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != data {
			t.Errorf("file = %q, want %q", got, data)
		}
	}
	if entries, _ := os.ReadDir(filepath.Dir(path)); len(entries) != 1 {
		t.Errorf("%d files in the directory, want only the thumbnail", len(entries))
	}
}
