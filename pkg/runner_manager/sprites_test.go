package runner_manager

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

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

func spriteRequest(version protobuf.StreamVersion, data string, interval uint32) *protobuf.SpriteReadyNotification {
	return &protobuf.SpriteReadyNotification{
		Stream:          &protobuf.StreamInfo{Id: ptr.Take(uint64(5))},
		StreamVersion:   ptr.Take(version),
		Sprite:          []byte(data),
		IntervalSeconds: ptr.Take(interval),
	}
}

// The file type must be the one the watch page looks for, and not the large thumbnail's.
func TestSaveSprite(t *testing.T) {
	stream := model.Stream{Model: gorm.Model{ID: 5}, CourseID: 2, Start: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)}
	for name, tc := range map[string]struct {
		version protobuf.StreamVersion
		source  string
	}{
		"combined":     {protobuf.StreamVersion_STREAM_VERSION_COMBINED, "COMB"},
		"camera":       {protobuf.StreamVersion_STREAM_VERSION_CAMERA, "CAM"},
		"presentation": {protobuf.StreamVersion_STREAM_VERSION_PRESENTATION, "PRES"},
	} {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			streams := mock_dao.NewMockStreamsDao(ctrl)
			files := mock_dao.NewMockFileDao(ctrl)
			streams.EXPECT().GetStreamByID(gomock.Any(), "5").Return(stream, nil)
			streams.EXPECT().SetThumbInterval(uint(5), uint32(30)).Return(nil)
			var saved model.File
			files.EXPECT().SetThumbnail(uint(5), gomock.Any()).DoAndReturn(func(_ uint, f model.File) error {
				saved = f
				return nil
			})
			m := New(dao.DaoWrapper{StreamsDao: streams, FileDao: files})
			m.massStorage = t.TempDir()

			if err := m.saveSprite(context.Background(), spriteRequest(tc.version, "jpeg", 30)); err != nil {
				t.Fatal(err)
			}
			got, err := os.ReadFile(saved.Path)
			if err != nil || string(got) != "jpeg" {
				t.Errorf("file %q holds %q, %v", saved.Path, got, err)
			}
			watched := model.Stream{Model: gorm.Model{ID: 5}, Files: []model.File{{Model: gorm.Model{ID: 9}, Type: saved.Type}}}
			if id := watched.GetThumbIdForSource(tc.source); id != 9 {
				t.Errorf("the watch page finds file %d for %s, want 9", id, tc.source)
			}
		})
	}

	t.Run("a sprite is dropped rather than break another version's interval", func(t *testing.T) {
		older := stream
		older.ThumbInterval = 60
		older.Files = []model.File{{Type: model.FILETYPE_THUMB_PRES}}
		ctrl := gomock.NewController(t)
		streams := mock_dao.NewMockStreamsDao(ctrl)
		streams.EXPECT().GetStreamByID(gomock.Any(), "5").Return(older, nil)
		// no SetThumbInterval or SetThumbnail: the mocks fail on an unexpected call
		m := New(dao.DaoWrapper{StreamsDao: streams, FileDao: mock_dao.NewMockFileDao(ctrl)})
		m.massStorage = t.TempDir()
		if err := m.saveSprite(context.Background(), spriteRequest(protobuf.StreamVersion_STREAM_VERSION_CAMERA, "jpeg", 30)); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("a report that can't be used is rejected for good", func(t *testing.T) {
		m := New(dao.DaoWrapper{StreamsDao: mock_dao.NewMockStreamsDao(gomock.NewController(t))})
		for _, req := range []*protobuf.SpriteReadyNotification{
			spriteRequest(protobuf.StreamVersion_STREAM_VERSION_UNSPECIFIED, "jpeg", 30),
			spriteRequest(protobuf.StreamVersion_STREAM_VERSION_COMBINED, "", 30),
			spriteRequest(protobuf.StreamVersion_STREAM_VERSION_COMBINED, "jpeg", 0),
		} {
			if got := status.Code(m.saveSprite(context.Background(), req)); got != codes.InvalidArgument {
				t.Errorf("code = %v, want InvalidArgument", got)
			}
		}
	})

	t.Run("a stream that is gone is not retried, a database that is down is", func(t *testing.T) {
		for lookup, want := range map[error]codes.Code{
			gorm.ErrRecordNotFound:               codes.NotFound,
			errors.New("driver: bad connection"): codes.Internal,
		} {
			streams := mock_dao.NewMockStreamsDao(gomock.NewController(t))
			streams.EXPECT().GetStreamByID(gomock.Any(), "5").Return(model.Stream{}, lookup)
			m := New(dao.DaoWrapper{StreamsDao: streams})
			if got := status.Code(m.saveSprite(context.Background(), spriteRequest(protobuf.StreamVersion_STREAM_VERSION_COMBINED, "jpeg", 30))); got != want {
				t.Errorf("%v: code = %v, want %v", lookup, got, want)
			}
		}
	})
}

func TestNeedsSprite(t *testing.T) {
	for name, tc := range map[string]struct {
		err  error
		want bool
	}{
		"has one":                     {nil, false},
		"has none":                    {gorm.ErrRecordNotFound, true},
		"the database is unreachable": {errors.New("driver: bad connection"), false},
	} {
		t.Run(name, func(t *testing.T) {
			files := mock_dao.NewMockFileDao(gomock.NewController(t))
			files.EXPECT().GetThumbnail(uint(5), model.FileType(model.FILETYPE_THUMB_PRES)).Return(model.File{}, tc.err)
			m := New(dao.DaoWrapper{FileDao: files})
			if got := m.needsSprite(5, model.PRES); got != tc.want {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}
