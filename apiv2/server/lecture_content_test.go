package apiv2

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"go.uber.org/mock/gomock"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"gorm.io/gorm"

	protobuf "github.com/TUM-Dev/gocast/apiv2/protobuf/server"
	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/mock_dao"
	"github.com/TUM-Dev/gocast/model"
	"github.com/TUM-Dev/gocast/voice-service/pb"
)

// contentMocks are the DAOs the lecture content handlers reach. As with lectureMocks,
// every expectation is explicit, so a refusal that wrote anything fails its test.
type contentMocks struct {
	streams  *mock_dao.MockStreamsDao
	courses  *mock_dao.MockCoursesDao
	sections *mock_dao.MockVideoSectionDao
	files    *mock_dao.MockFileDao
}

func newContentAPI(t *testing.T) (*API, contentMocks) {
	ctrl := gomock.NewController(t)
	m := contentMocks{
		streams:  mock_dao.NewMockStreamsDao(ctrl),
		courses:  mock_dao.NewMockCoursesDao(ctrl),
		sections: mock_dao.NewMockVideoSectionDao(ctrl),
		files:    mock_dao.NewMockFileDao(ctrl),
	}
	return &API{
		dao: dao.DaoWrapper{
			StreamsDao: m.streams, CoursesDao: m.courses, VideoSectionDao: m.sections, FileDao: m.files,
		},
		log: slog.Default(),
	}, m
}

// noSigning stands in for the playlist signing, which needs the server's key.
func noSigning(t *testing.T) {
	t.Helper()
	orig := signPlaylists
	signPlaylists = func(s *model.Stream, _ *model.User, _ bool) error {
		if s.PlaylistUrl != "" {
			s.PlaylistUrl += "?jwt=signed"
		}
		if s.PlaylistUrlPRES != "" {
			s.PlaylistUrlPRES += "?jwt=signed"
		}
		return nil
	}
	t.Cleanup(func() { signPlaylists = orig })
}

// recordedLecture is courseOneLecture with a recording.
func recordedLecture() model.Stream {
	s := courseOneLecture()
	s.Recording = true
	s.PlaylistUrl = "https://edge/comb.m3u8"
	return s
}

var courseOne = model.Course{Model: gorm.Model{ID: 1}, Name: "Course", Year: 2026, TeachingTerm: "W"}

type generated struct {
	streamID uint
	playlist string
	course   model.Course
	sections []model.VideoSection
}

// fakeSectionImages records what it is asked for. The handlers call it from a
// goroutine, hence the channels.
type fakeSectionImages struct {
	generated chan generated
	deleted   chan string
}

func newFakeSectionImages() *fakeSectionImages {
	return &fakeSectionImages{generated: make(chan generated, 4), deleted: make(chan string, 4)}
}

func (f *fakeSectionImages) Generate(streamID uint, playlistURL string, course model.Course, sections []model.VideoSection) error {
	f.generated <- generated{streamID, playlistURL, course, sections}
	return nil
}

func (f *fakeSectionImages) Delete(path string) error {
	f.deleted <- path
	return nil
}

func receive[T any](t *testing.T, ch <-chan T) T {
	t.Helper()
	select {
	case v := <-ch:
		return v
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for the background call")
		var zero T
		return zero
	}
}

func TestCreateLectureSections(t *testing.T) {
	t.Run("creates them on the lecture whatever was sent and has thumbnails made", func(t *testing.T) {
		noSigning(t)
		api, m := newContentAPI(t)
		images := newFakeSectionImages()
		api.sectionImages = images
		all := []model.VideoSection{
			{Model: gorm.Model{ID: 1}, StreamID: 7, Description: "Intro"},
			{Model: gorm.Model{ID: 2}, StreamID: 7, Description: "Proofs", StartHours: 1, StartMinutes: 2, StartSeconds: 3},
		}
		m.streams.EXPECT().GetStreamByID(gomock.Any(), "7").Return(recordedLecture(), nil)
		m.sections.EXPECT().Create(gomock.Any()).DoAndReturn(func(s []model.VideoSection) error {
			if len(s) != 1 || s[0].StreamID != 7 || s[0].Description != "Proofs" || s[0].StartHours != 1 || s[0].StartSeconds != 3 {
				t.Errorf("created %+v", s)
			}
			return nil
		})
		m.sections.EXPECT().GetByStreamId(uint(7)).Return(all, nil)
		m.courses.EXPECT().GetCourseById(gomock.Any(), uint(1)).Return(courseOne, nil)

		resp, err := api.CreateLectureSections(asCaller(lectureAdmin), &protobuf.CreateLectureSectionsRequest{
			CourseId: 1, StreamId: 7,
			Sections: []*protobuf.LectureSectionInput{{Description: " Proofs ", StartHours: 1, StartMinutes: 2, StartSeconds: 3}},
		})
		if err != nil {
			t.Fatalf("CreateLectureSections: %v", err)
		}
		if len(resp.Sections) != 2 || resp.Sections[1].Id != 2 || resp.Sections[1].StartMinutes != 2 {
			t.Errorf("sections = %v", resp.Sections)
		}
		got := receive(t, images.generated)
		if got.streamID != 7 || got.playlist != "https://edge/comb.m3u8?jwt=signed" || got.course.Name != "Course" || len(got.sections) != 2 {
			t.Errorf("generated %+v", got)
		}
	})

	t.Run("makes no thumbnails for a lecture without a recording", func(t *testing.T) {
		api, m := newContentAPI(t)
		api.sectionImages = newFakeSectionImages()
		m.streams.EXPECT().GetStreamByID(gomock.Any(), "7").Return(courseOneLecture(), nil)
		m.sections.EXPECT().Create(gomock.Any()).Return(nil)
		m.sections.EXPECT().GetByStreamId(uint(7)).Return([]model.VideoSection{{Model: gorm.Model{ID: 1}}}, nil)

		if _, err := api.CreateLectureSections(asCaller(lectureAdmin), &protobuf.CreateLectureSectionsRequest{
			CourseId: 1, StreamId: 7, Sections: []*protobuf.LectureSectionInput{{Description: "Intro"}},
		}); err != nil {
			t.Fatalf("CreateLectureSections: %v", err)
		}
	})

	for name, sections := range map[string][]*protobuf.LectureSectionInput{
		"no sections":         nil,
		"a blank description": {{Description: "  "}},
		"60 minutes":          {{Description: "x", StartMinutes: 60}},
		"60 seconds":          {{Description: "x", StartSeconds: 60}},
		"one bad among good":  {{Description: "ok"}, {Description: "x", StartSeconds: 99}},
	} {
		t.Run("rejects "+name+" and writes nothing", func(t *testing.T) {
			api, m := newContentAPI(t)
			m.streams.EXPECT().GetStreamByID(gomock.Any(), "7").Return(courseOneLecture(), nil)
			_, err := api.CreateLectureSections(asCaller(lectureAdmin), &protobuf.CreateLectureSectionsRequest{
				CourseId: 1, StreamId: 7, Sections: sections,
			})
			wantCode(t, err, codes.InvalidArgument)
		})
	}

	t.Run("404s another course's lecture and writes nothing", func(t *testing.T) {
		api, m := newContentAPI(t)
		m.streams.EXPECT().GetStreamByID(gomock.Any(), "7").Return(foreignLecture(), nil)
		_, err := api.CreateLectureSections(asCaller(lectureAdmin), &protobuf.CreateLectureSectionsRequest{
			CourseId: 1, StreamId: 7, Sections: []*protobuf.LectureSectionInput{{Description: "Intro"}},
		})
		wantCode(t, err, codes.NotFound)
	})
}

func TestUpdateLectureSection(t *testing.T) {
	existing := func() model.VideoSection {
		return model.VideoSection{Model: gorm.Model{ID: 3}, StreamID: 7, Description: "Old", StartHours: 1, StartMinutes: 5}
	}

	t.Run("moves a section to 0:00:00, which v1 could not, and remakes its thumbnail", func(t *testing.T) {
		noSigning(t)
		api, m := newContentAPI(t)
		images := newFakeSectionImages()
		api.sectionImages = images
		m.streams.EXPECT().GetStreamByID(gomock.Any(), "7").Return(recordedLecture(), nil)
		m.sections.EXPECT().Get(uint(3)).Return(existing(), nil)
		m.sections.EXPECT().UpdateContent(gomock.Any()).DoAndReturn(func(s *model.VideoSection) error {
			if s.ID != 3 || s.Description != "New" || s.StartHours != 0 || s.StartMinutes != 0 || s.StartSeconds != 0 {
				t.Errorf("updated %+v", s)
			}
			return nil
		})
		m.courses.EXPECT().GetCourseById(gomock.Any(), uint(1)).Return(courseOne, nil)

		if _, err := api.UpdateLectureSection(asCaller(lectureAdmin), &protobuf.UpdateLectureSectionRequest{
			CourseId: 1, StreamId: 7, SectionId: 3, Description: "New",
		}); err != nil {
			t.Fatalf("UpdateLectureSection: %v", err)
		}
		got := receive(t, images.generated)
		if len(got.sections) != 1 || got.sections[0].ID != 3 {
			t.Errorf("generated %+v", got)
		}
	})

	t.Run("keeps the thumbnail of a section that did not move", func(t *testing.T) {
		api, m := newContentAPI(t)
		api.sectionImages = newFakeSectionImages()
		m.streams.EXPECT().GetStreamByID(gomock.Any(), "7").Return(recordedLecture(), nil)
		m.sections.EXPECT().Get(uint(3)).Return(existing(), nil)
		m.sections.EXPECT().UpdateContent(gomock.Any()).Return(nil)
		// No course lookup: nothing is generated.

		if _, err := api.UpdateLectureSection(asCaller(lectureAdmin), &protobuf.UpdateLectureSectionRequest{
			CourseId: 1, StreamId: 7, SectionId: 3, Description: "Renamed", StartHours: 1, StartMinutes: 5,
		}); err != nil {
			t.Fatalf("UpdateLectureSection: %v", err)
		}
	})

	t.Run("404s another lecture's section and writes nothing", func(t *testing.T) {
		api, m := newContentAPI(t)
		other := existing()
		other.StreamID = 8
		m.streams.EXPECT().GetStreamByID(gomock.Any(), "7").Return(courseOneLecture(), nil)
		m.sections.EXPECT().Get(uint(3)).Return(other, nil)

		_, err := api.UpdateLectureSection(asCaller(lectureAdmin), &protobuf.UpdateLectureSectionRequest{
			CourseId: 1, StreamId: 7, SectionId: 3, Description: "New",
		})
		wantCode(t, err, codes.NotFound)
	})

	t.Run("404s a missing section", func(t *testing.T) {
		api, m := newContentAPI(t)
		m.streams.EXPECT().GetStreamByID(gomock.Any(), "7").Return(courseOneLecture(), nil)
		m.sections.EXPECT().Get(uint(3)).Return(model.VideoSection{}, nil)

		_, err := api.UpdateLectureSection(asCaller(lectureAdmin), &protobuf.UpdateLectureSectionRequest{
			CourseId: 1, StreamId: 7, SectionId: 3, Description: "New",
		})
		wantCode(t, err, codes.NotFound)
	})

	t.Run("rejects 60 seconds and writes nothing", func(t *testing.T) {
		api, m := newContentAPI(t)
		m.streams.EXPECT().GetStreamByID(gomock.Any(), "7").Return(courseOneLecture(), nil)
		_, err := api.UpdateLectureSection(asCaller(lectureAdmin), &protobuf.UpdateLectureSectionRequest{
			CourseId: 1, StreamId: 7, SectionId: 3, Description: "New", StartSeconds: 60,
		})
		wantCode(t, err, codes.InvalidArgument)
	})

	t.Run("404s another course's lecture and writes nothing", func(t *testing.T) {
		api, m := newContentAPI(t)
		m.streams.EXPECT().GetStreamByID(gomock.Any(), "7").Return(foreignLecture(), nil)
		_, err := api.UpdateLectureSection(asCaller(lectureAdmin), &protobuf.UpdateLectureSectionRequest{
			CourseId: 1, StreamId: 7, SectionId: 3, Description: "New",
		})
		wantCode(t, err, codes.NotFound)
	})
}

func TestDeleteLectureSection(t *testing.T) {
	t.Run("deletes it and has a worker delete its thumbnail", func(t *testing.T) {
		api, m := newContentAPI(t)
		images := newFakeSectionImages()
		api.sectionImages = images
		m.streams.EXPECT().GetStreamByID(gomock.Any(), "7").Return(courseOneLecture(), nil)
		m.sections.EXPECT().Get(uint(3)).Return(model.VideoSection{Model: gorm.Model{ID: 3}, StreamID: 7, FileID: 12}, nil)
		m.sections.EXPECT().Delete(uint(3)).Return(nil)
		m.files.EXPECT().GetFileById("12").Return(model.File{Model: gorm.Model{ID: 12}, Path: "/mass/sections/7_3.jpg"}, nil)

		if _, err := api.DeleteLectureSection(asCaller(lectureAdmin), &protobuf.DeleteLectureSectionRequest{
			CourseId: 1, StreamId: 7, SectionId: 3,
		}); err != nil {
			t.Fatalf("DeleteLectureSection: %v", err)
		}
		if got := receive(t, images.deleted); got != "/mass/sections/7_3.jpg" {
			t.Errorf("deleted %q", got)
		}
	})

	t.Run("asks for nothing when the section had no thumbnail", func(t *testing.T) {
		api, m := newContentAPI(t)
		api.sectionImages = newFakeSectionImages()
		m.streams.EXPECT().GetStreamByID(gomock.Any(), "7").Return(courseOneLecture(), nil)
		m.sections.EXPECT().Get(uint(3)).Return(model.VideoSection{Model: gorm.Model{ID: 3}, StreamID: 7}, nil)
		m.sections.EXPECT().Delete(uint(3)).Return(nil)

		if _, err := api.DeleteLectureSection(asCaller(lectureAdmin), &protobuf.DeleteLectureSectionRequest{
			CourseId: 1, StreamId: 7, SectionId: 3,
		}); err != nil {
			t.Fatalf("DeleteLectureSection: %v", err)
		}
	})

	t.Run("404s another lecture's section and deletes nothing", func(t *testing.T) {
		api, m := newContentAPI(t)
		m.streams.EXPECT().GetStreamByID(gomock.Any(), "7").Return(courseOneLecture(), nil)
		m.sections.EXPECT().Get(uint(3)).Return(model.VideoSection{Model: gorm.Model{ID: 3}, StreamID: 8}, nil)

		_, err := api.DeleteLectureSection(asCaller(lectureAdmin), &protobuf.DeleteLectureSectionRequest{
			CourseId: 1, StreamId: 7, SectionId: 3,
		})
		wantCode(t, err, codes.NotFound)
	})

	t.Run("404s another course's lecture and deletes nothing", func(t *testing.T) {
		api, m := newContentAPI(t)
		m.streams.EXPECT().GetStreamByID(gomock.Any(), "7").Return(foreignLecture(), nil)
		_, err := api.DeleteLectureSection(asCaller(lectureAdmin), &protobuf.DeleteLectureSectionRequest{
			CourseId: 1, StreamId: 7, SectionId: 3,
		})
		wantCode(t, err, codes.NotFound)
	})
}

func writeTempFile(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "slides.pdf")
	if err := os.WriteFile(path, []byte("%PDF"), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestDeleteLectureAttachment(t *testing.T) {
	t.Run("deletes the row and the file", func(t *testing.T) {
		api, m := newContentAPI(t)
		path := writeTempFile(t)
		m.streams.EXPECT().GetStreamByID(gomock.Any(), "7").Return(courseOneLecture(), nil)
		m.files.EXPECT().GetFileById("5").Return(model.File{Model: gorm.Model{ID: 5}, StreamID: 7, Path: path, Type: model.FILETYPE_ATTACHMENT}, nil)
		m.files.EXPECT().DeleteFile(uint(5)).Return(nil)

		if _, err := api.DeleteLectureAttachment(asCaller(lectureAdmin), &protobuf.DeleteLectureAttachmentRequest{
			CourseId: 1, StreamId: 7, FileId: 5,
		}); err != nil {
			t.Fatalf("DeleteLectureAttachment: %v", err)
		}
		if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("file still there: %v", err)
		}
	})

	// v1 answered 500 for both and kept the row for good.
	for name, path := range map[string]string{
		"a file already gone from disk":    filepath.Join(t.TempDir(), "gone.pdf"),
		"an attachment by URL (pre-#1841)": "https://example.com/slides.pdf",
	} {
		t.Run("deletes the row of "+name, func(t *testing.T) {
			api, m := newContentAPI(t)
			m.streams.EXPECT().GetStreamByID(gomock.Any(), "7").Return(courseOneLecture(), nil)
			m.files.EXPECT().GetFileById("5").Return(model.File{Model: gorm.Model{ID: 5}, StreamID: 7, Path: path, Type: model.FILETYPE_ATTACHMENT}, nil)
			m.files.EXPECT().DeleteFile(uint(5)).Return(nil)

			if _, err := api.DeleteLectureAttachment(asCaller(lectureAdmin), &protobuf.DeleteLectureAttachmentRequest{
				CourseId: 1, StreamId: 7, FileId: 5,
			}); err != nil {
				t.Fatalf("DeleteLectureAttachment: %v", err)
			}
		})
	}

	for name, file := range map[string]model.File{
		"another lecture's attachment": {Model: gorm.Model{ID: 5}, StreamID: 8, Type: model.FILETYPE_ATTACHMENT},
		"the lecture's recording":      {Model: gorm.Model{ID: 5}, StreamID: 7, Type: model.FILETYPE_VOD},
		"the lecture's thumbnail":      {Model: gorm.Model{ID: 5}, StreamID: 7, Type: model.FILETYPE_THUMB_CUSTOM},
	} {
		t.Run("404s "+name+" and deletes nothing", func(t *testing.T) {
			api, m := newContentAPI(t)
			file.Path = writeTempFile(t)
			m.streams.EXPECT().GetStreamByID(gomock.Any(), "7").Return(courseOneLecture(), nil)
			m.files.EXPECT().GetFileById("5").Return(file, nil)

			_, err := api.DeleteLectureAttachment(asCaller(lectureAdmin), &protobuf.DeleteLectureAttachmentRequest{
				CourseId: 1, StreamId: 7, FileId: 5,
			})
			wantCode(t, err, codes.NotFound)
			if _, err := os.Stat(file.Path); err != nil {
				t.Errorf("file was touched: %v", err)
			}
		})
	}

	t.Run("404s a missing file", func(t *testing.T) {
		api, m := newContentAPI(t)
		m.streams.EXPECT().GetStreamByID(gomock.Any(), "7").Return(courseOneLecture(), nil)
		m.files.EXPECT().GetFileById("5").Return(model.File{}, gorm.ErrRecordNotFound)
		_, err := api.DeleteLectureAttachment(asCaller(lectureAdmin), &protobuf.DeleteLectureAttachmentRequest{
			CourseId: 1, StreamId: 7, FileId: 5,
		})
		wantCode(t, err, codes.NotFound)
	})

	t.Run("404s another course's lecture and deletes nothing", func(t *testing.T) {
		api, m := newContentAPI(t)
		m.streams.EXPECT().GetStreamByID(gomock.Any(), "7").Return(foreignLecture(), nil)
		_, err := api.DeleteLectureAttachment(asCaller(lectureAdmin), &protobuf.DeleteLectureAttachmentRequest{
			CourseId: 1, StreamId: 7, FileId: 5,
		})
		wantCode(t, err, codes.NotFound)
	})
}

func TestDeleteLectureThumbnail(t *testing.T) {
	t.Run("deletes the custom thumbnail and its file", func(t *testing.T) {
		api, m := newContentAPI(t)
		path := writeTempFile(t)
		m.streams.EXPECT().GetStreamByID(gomock.Any(), "7").Return(courseOneLecture(), nil)
		m.files.EXPECT().GetThumbnail(uint(7), model.FileType(model.FILETYPE_THUMB_CUSTOM)).
			Return(model.File{Model: gorm.Model{ID: 6}, StreamID: 7, Path: path, Type: model.FILETYPE_THUMB_CUSTOM}, nil)
		m.files.EXPECT().DeleteFile(uint(6)).Return(nil)

		if _, err := api.DeleteLectureThumbnail(asCaller(lectureAdmin), &protobuf.DeleteLectureThumbnailRequest{CourseId: 1, StreamId: 7}); err != nil {
			t.Fatalf("DeleteLectureThumbnail: %v", err)
		}
		if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("file still there: %v", err)
		}
	})

	t.Run("404s a lecture without one", func(t *testing.T) {
		api, m := newContentAPI(t)
		m.streams.EXPECT().GetStreamByID(gomock.Any(), "7").Return(courseOneLecture(), nil)
		m.files.EXPECT().GetThumbnail(uint(7), model.FileType(model.FILETYPE_THUMB_CUSTOM)).Return(model.File{}, gorm.ErrRecordNotFound)
		_, err := api.DeleteLectureThumbnail(asCaller(lectureAdmin), &protobuf.DeleteLectureThumbnailRequest{CourseId: 1, StreamId: 7})
		wantCode(t, err, codes.NotFound)
	})

	t.Run("404s another course's lecture and deletes nothing", func(t *testing.T) {
		api, m := newContentAPI(t)
		m.streams.EXPECT().GetStreamByID(gomock.Any(), "7").Return(foreignLecture(), nil)
		_, err := api.DeleteLectureThumbnail(asCaller(lectureAdmin), &protobuf.DeleteLectureThumbnailRequest{CourseId: 1, StreamId: 7})
		wantCode(t, err, codes.NotFound)
	})
}

// fakeVoiceService records the one Generate it expects.
type fakeVoiceService struct {
	req  *pb.GenerateRequest
	auth []string
	err  error
}

func (f *fakeVoiceService) Generate(ctx context.Context, in *pb.GenerateRequest, _ ...grpc.CallOption) (*emptypb.Empty, error) {
	f.req = in
	md, _ := metadata.FromOutgoingContext(ctx)
	f.auth = md.Get("auth")
	return &emptypb.Empty{}, f.err
}

func TestRequestLectureSubtitles(t *testing.T) {
	t.Run("hands the voice service the signed recording", func(t *testing.T) {
		noSigning(t)
		api, m := newContentAPI(t)
		voice := &fakeVoiceService{}
		WithSubtitleGenerator(voice, "token")(api)
		m.streams.EXPECT().GetStreamByID(gomock.Any(), "7").Return(recordedLecture(), nil)

		if _, err := api.RequestLectureSubtitles(asCaller(lectureAdmin), &protobuf.RequestLectureSubtitlesRequest{
			CourseId: 1, StreamId: 7, Language: "de",
		}); err != nil {
			t.Fatalf("RequestLectureSubtitles: %v", err)
		}
		if voice.req.GetStreamId() != 7 || voice.req.GetLanguage() != "de" || voice.req.GetSourceFile() != "https://edge/comb.m3u8?jwt=signed" {
			t.Errorf("request = %v", voice.req)
		}
		if len(voice.auth) != 1 || voice.auth[0] != "token" {
			t.Errorf("auth = %v", voice.auth)
		}
	})

	t.Run("falls back to the presentation recording", func(t *testing.T) {
		noSigning(t)
		api, m := newContentAPI(t)
		voice := &fakeVoiceService{}
		WithSubtitleGenerator(voice, "")(api)
		s := courseOneLecture()
		s.PlaylistUrlPRES = "https://edge/pres.m3u8"
		m.streams.EXPECT().GetStreamByID(gomock.Any(), "7").Return(s, nil)

		if _, err := api.RequestLectureSubtitles(asCaller(lectureAdmin), &protobuf.RequestLectureSubtitlesRequest{
			CourseId: 1, StreamId: 7, Language: "en",
		}); err != nil {
			t.Fatalf("RequestLectureSubtitles: %v", err)
		}
		if voice.req.GetSourceFile() != "https://edge/pres.m3u8?jwt=signed" || len(voice.auth) != 0 {
			t.Errorf("request = %v, auth = %v", voice.req, voice.auth)
		}
	})

	t.Run("503s without a voice service", func(t *testing.T) {
		api, m := newContentAPI(t)
		m.streams.EXPECT().GetStreamByID(gomock.Any(), "7").Return(recordedLecture(), nil)
		_, err := api.RequestLectureSubtitles(asCaller(lectureAdmin), &protobuf.RequestLectureSubtitlesRequest{
			CourseId: 1, StreamId: 7, Language: "de",
		})
		wantCode(t, err, codes.Unavailable)
	})

	t.Run("503s an unreachable voice service", func(t *testing.T) {
		noSigning(t)
		api, m := newContentAPI(t)
		WithSubtitleGenerator(&fakeVoiceService{err: status.Error(codes.Unavailable, "down")}, "")(api)
		m.streams.EXPECT().GetStreamByID(gomock.Any(), "7").Return(recordedLecture(), nil)
		_, err := api.RequestLectureSubtitles(asCaller(lectureAdmin), &protobuf.RequestLectureSubtitlesRequest{
			CourseId: 1, StreamId: 7, Language: "de",
		})
		wantCode(t, err, codes.Unavailable)
	})

	t.Run("400s a lecture without a recording", func(t *testing.T) {
		noSigning(t)
		api, m := newContentAPI(t)
		voice := &fakeVoiceService{}
		WithSubtitleGenerator(voice, "")(api)
		m.streams.EXPECT().GetStreamByID(gomock.Any(), "7").Return(courseOneLecture(), nil)
		_, err := api.RequestLectureSubtitles(asCaller(lectureAdmin), &protobuf.RequestLectureSubtitlesRequest{
			CourseId: 1, StreamId: 7, Language: "de",
		})
		wantCode(t, err, codes.InvalidArgument)
		if voice.req != nil {
			t.Errorf("voice service asked: %v", voice.req)
		}
	})

	for _, lang := range []string{"", "german", "DE", "d"} {
		t.Run("400s the language "+lang, func(t *testing.T) {
			api, m := newContentAPI(t)
			WithSubtitleGenerator(&fakeVoiceService{}, "")(api)
			m.streams.EXPECT().GetStreamByID(gomock.Any(), "7").Return(recordedLecture(), nil)
			_, err := api.RequestLectureSubtitles(asCaller(lectureAdmin), &protobuf.RequestLectureSubtitlesRequest{
				CourseId: 1, StreamId: 7, Language: lang,
			})
			wantCode(t, err, codes.InvalidArgument)
		})
	}

	t.Run("404s another course's lecture", func(t *testing.T) {
		api, m := newContentAPI(t)
		voice := &fakeVoiceService{}
		WithSubtitleGenerator(voice, "")(api)
		m.streams.EXPECT().GetStreamByID(gomock.Any(), "7").Return(foreignLecture(), nil)
		_, err := api.RequestLectureSubtitles(asCaller(lectureAdmin), &protobuf.RequestLectureSubtitlesRequest{
			CourseId: 1, StreamId: 7, Language: "de",
		})
		wantCode(t, err, codes.NotFound)
		if voice.req != nil {
			t.Errorf("voice service asked: %v", voice.req)
		}
	})
}

func TestGetLectureTranscodingProgress(t *testing.T) {
	t.Run("lists the versions still transcoding", func(t *testing.T) {
		api, m := newContentAPI(t)
		m.streams.EXPECT().GetStreamByID(gomock.Any(), "7").Return(courseOneLecture(), nil)
		m.streams.EXPECT().GetTranscodingProgressByVersion(model.COMB, uint(7)).Return(model.TranscodingProgress{}, gorm.ErrRecordNotFound)
		m.streams.EXPECT().GetTranscodingProgressByVersion(model.PRES, uint(7)).
			Return(model.TranscodingProgress{StreamID: 7, Version: model.PRES, Progress: 40}, nil)
		m.streams.EXPECT().GetTranscodingProgressByVersion(model.CAM, uint(7)).Return(model.TranscodingProgress{}, gorm.ErrRecordNotFound)

		resp, err := api.GetLectureTranscodingProgress(asCaller(lectureAdmin), &protobuf.GetLectureTranscodingProgressRequest{CourseId: 1, StreamId: 7})
		if err != nil {
			t.Fatalf("GetLectureTranscodingProgress: %v", err)
		}
		if len(resp.Progresses) != 1 || resp.Progresses[0].Version != "PRES" || resp.Progresses[0].Progress != 40 {
			t.Errorf("progresses = %v", resp.Progresses)
		}
	})

	t.Run("500s a failed read", func(t *testing.T) {
		api, m := newContentAPI(t)
		m.streams.EXPECT().GetStreamByID(gomock.Any(), "7").Return(courseOneLecture(), nil)
		m.streams.EXPECT().GetTranscodingProgressByVersion(model.COMB, uint(7)).Return(model.TranscodingProgress{}, errors.New("db down"))
		_, err := api.GetLectureTranscodingProgress(asCaller(lectureAdmin), &protobuf.GetLectureTranscodingProgressRequest{CourseId: 1, StreamId: 7})
		wantCode(t, err, codes.Unknown)
	})

	t.Run("404s another course's lecture", func(t *testing.T) {
		api, m := newContentAPI(t)
		m.streams.EXPECT().GetStreamByID(gomock.Any(), "7").Return(foreignLecture(), nil)
		_, err := api.GetLectureTranscodingProgress(asCaller(lectureAdmin), &protobuf.GetLectureTranscodingProgressRequest{CourseId: 1, StreamId: 7})
		wantCode(t, err, codes.NotFound)
	})
}
