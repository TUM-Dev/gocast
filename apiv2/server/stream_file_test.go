package apiv2

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.uber.org/mock/gomock"
	"gorm.io/gorm"

	"github.com/TUM-Dev/gocast/model"
)

func TestStreamFileRouteOf(t *testing.T) {
	for _, tc := range []struct {
		method, path string
		want         *streamFileRoute
	}{
		{"GET", "/api/v2/streams/7/files/42", &streamFileRoute{streamID: 7, fileID: 42}},
		{"HEAD", "/api/v2/streams/7/files/42", &streamFileRoute{streamID: 7, fileID: 42}},
		{"POST", "/api/v2/streams/7/files/42", nil},
		{"GET", "/api/v2/streams/7/files", nil},
		{"GET", "/api/v2/streams/7/files/42/x", nil},
		{"GET", "/api/v2/streams/x/files/42", nil},
		{"GET", "/api/v2/streams/7/chat/42", nil},
		{"GET", "/api/v2/courses/7/files/42", nil},
	} {
		got, ok := routed(streamFilePath, tc.method, tc.path, streamFileRouteOf)
		if (tc.want == nil) == ok || (tc.want != nil && got != *tc.want) {
			t.Errorf("%s %s: got %+v, %v; want %+v", tc.method, tc.path, got, ok, tc.want)
		}
	}
}

func serveStreamFile(api *API, req *http.Request) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	api.Handler().ServeHTTP(rec, req)
	return rec
}

func TestServeStreamFile(t *testing.T) {
	dir := t.TempDir()
	write := func(name string) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte("0123456789"), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	attachment := write("slides.pdf")
	thumb := write("thumb.jpg")
	vod := write("comb.mp4")

	enrolledStudent := &model.User{Model: gorm.Model{ID: 10}, Role: model.StudentType, Courses: []model.Course{{Model: gorm.Model{ID: 1}}}}
	otherStudent := &model.User{Model: gorm.Model{ID: 11}, Role: model.StudentType}

	course := func(visibility string, downloads bool) model.Course {
		c := courseOne
		c.Visibility = visibility
		c.DownloadsEnabled = downloads
		return c
	}
	stream := func(mod func(*model.Stream)) model.Stream {
		s := recordedLecture()
		if mod != nil {
			mod(&s)
		}
		return s
	}
	private := func(s *model.Stream) { s.Private = true }
	notRecorded := func(s *model.Stream) { s.Recording, s.PlaylistUrl = false, "" }

	file := func(path string, typ model.FileType, streamID uint) model.File {
		return model.File{Model: gorm.Model{ID: 42}, StreamID: streamID, Path: path, Filename: filepath.Base(path), Type: typ}
	}

	for _, tc := range []struct {
		name   string
		course model.Course
		stream model.Stream
		file   model.File
		user   *model.User
		want   int
	}{
		{"anonymous, public course, thumbnail", course("public", false), stream(nil), file(thumb, model.FILETYPE_THUMB_COMB, 7), nil, 200},
		{"anonymous, public course, custom thumbnail", course("public", false), stream(nil), file(thumb, model.FILETYPE_THUMB_CUSTOM, 7), nil, 200},
		{"anonymous, public course, section image", course("public", false), stream(nil), file(thumb, model.FILETYPE_IMAGE_JPG, 7), nil, 200},
		{"anonymous, public course, attachment, downloads off", course("public", false), stream(nil), file(attachment, model.FILETYPE_ATTACHMENT, 7), nil, 200},
		{"student, attachment, downloads off", course("public", false), stream(nil), file(attachment, model.FILETYPE_ATTACHMENT, 7), otherStudent, 200},

		{"foreign file", course("public", true), stream(nil), file(thumb, model.FILETYPE_THUMB_COMB, 8), nil, 404},
		{"foreign file, admin", course("public", true), stream(nil), file(thumb, model.FILETYPE_THUMB_COMB, 8), lectureAdmin, 404},
		{"invalid file type", course("public", true), stream(nil), file(thumb, model.FILETYPE_INVALID, 7), lectureAdmin, 404},
		{"file gone from disk", course("public", true), stream(nil), file(filepath.Join(dir, "gone.jpg"), model.FILETYPE_THUMB_COMB, 7), nil, 404},

		{"private stream, anonymous", course("public", true), stream(private), file(thumb, model.FILETYPE_THUMB_COMB, 7), nil, 404},
		{"private stream, student", course("public", true), stream(private), file(thumb, model.FILETYPE_THUMB_COMB, 7), enrolledStudent, 404},
		{"private stream, course admin", course("public", true), stream(private), file(thumb, model.FILETYPE_THUMB_COMB, 7), lectureAdmin, 200},

		{"logged-in-only course, anonymous", course("loggedin", true), stream(nil), file(thumb, model.FILETYPE_THUMB_COMB, 7), nil, 401},
		{"logged-in-only course, student", course("loggedin", true), stream(nil), file(thumb, model.FILETYPE_THUMB_COMB, 7), otherStudent, 200},
		{"enrolled-only course, anonymous", course("enrolled", true), stream(nil), file(attachment, model.FILETYPE_ATTACHMENT, 7), nil, 401},
		{"enrolled-only course, not enrolled", course("enrolled", true), stream(nil), file(attachment, model.FILETYPE_ATTACHMENT, 7), otherStudent, 404},
		{"enrolled-only course, enrolled", course("enrolled", true), stream(nil), file(attachment, model.FILETYPE_ATTACHMENT, 7), enrolledStudent, 200},
		{"enrolled-only course, admin", course("enrolled", true), stream(nil), file(attachment, model.FILETYPE_ATTACHMENT, 7), lectureAdmin, 200},

		{"recording, downloads on, anonymous", course("public", true), stream(nil), file(vod, model.FILETYPE_VOD, 7), nil, 401},
		{"recording, downloads on, student", course("public", true), stream(nil), file(vod, model.FILETYPE_VOD, 7), otherStudent, 200},
		{"recording, downloads off, anonymous", course("public", false), stream(nil), file(vod, model.FILETYPE_VOD, 7), nil, 404},
		{"recording, downloads off, student", course("public", false), stream(nil), file(vod, model.FILETYPE_VOD, 7), otherStudent, 404},
		{"recording, downloads off, course admin", course("public", false), stream(nil), file(vod, model.FILETYPE_VOD, 7), lectureAdmin, 200},
		{"recording, downloads off, server admin", course("public", false), stream(nil), file(vod, model.FILETYPE_VOD, 7), serverAdmin, 200},
		{"recording, not downloadable, admin", course("public", true), stream(notRecorded), file(vod, model.FILETYPE_VOD, 7), lectureAdmin, 404},
		{"recording, not enrolled in an enrolled course", course("enrolled", true), stream(nil), file(vod, model.FILETYPE_VOD, 7), otherStudent, 404},
	} {
		t.Run(tc.name, func(t *testing.T) {
			api, m := newContentAPI(t)
			m.streams.EXPECT().GetStreamByID(gomock.Any(), "7").Return(tc.stream, nil).AnyTimes()
			m.courses.EXPECT().GetCourseById(gomock.Any(), uint(1)).Return(tc.course, nil).AnyTimes()
			m.files.EXPECT().GetFileById("42").Return(tc.file, nil).AnyTimes()

			req := httptest.NewRequest(http.MethodGet, "/api/v2/streams/7/files/42", nil)
			if tc.user != nil {
				req = req.WithContext(asCaller(tc.user))
			}
			rec := serveStreamFile(api, req)
			if rec.Code != tc.want {
				t.Fatalf("status = %d, want %d: %s", rec.Code, tc.want, rec.Body)
			}
			if tc.want == 200 && rec.Body.String() != "0123456789" {
				t.Errorf("body = %q", rec.Body)
			}
			if tc.want != 200 && strings.Contains(rec.Body.String(), "0123456789") {
				t.Error("refusal leaked the file")
			}
		})
	}
}

func TestServeStreamFileDelivery(t *testing.T) {
	path := filepath.Join(t.TempDir(), "notes.pdf")
	if err := os.WriteFile(path, []byte("0123456789"), 0o644); err != nil {
		t.Fatal(err)
	}
	get := func(t *testing.T, query string, hdr map[string]string, name string) *httptest.ResponseRecorder {
		api, m := newContentAPI(t)
		c := courseOne
		c.Visibility = "public"
		m.streams.EXPECT().GetStreamByID(gomock.Any(), "7").Return(courseOneLecture(), nil)
		m.courses.EXPECT().GetCourseById(gomock.Any(), uint(1)).Return(c, nil)
		m.files.EXPECT().GetFileById("42").Return(model.File{
			Model: gorm.Model{ID: 42}, StreamID: 7, Path: path, Filename: name, Type: model.FILETYPE_ATTACHMENT,
		}, nil)
		req := httptest.NewRequest(http.MethodGet, "/api/v2/streams/7/files/42"+query, nil)
		for k, v := range hdr {
			req.Header.Set(k, v)
		}
		return serveStreamFile(api, req)
	}

	t.Run("inline by default", func(t *testing.T) {
		rec := get(t, "", nil, "notes.pdf")
		if got := rec.Header().Get("Content-Type"); got != "application/pdf" {
			t.Errorf("type = %q", got)
		}
		if got := rec.Header().Get("Content-Disposition"); got != `inline; filename=notes.pdf` {
			t.Errorf("disposition = %q", got)
		}
	})
	t.Run("attachment when asked", func(t *testing.T) {
		rec := get(t, "?download=1", nil, "notes.pdf")
		if got := rec.Header().Get("Content-Disposition"); !strings.HasPrefix(got, "attachment;") {
			t.Errorf("disposition = %q", got)
		}
	})
	t.Run("a type that could run script is never inline", func(t *testing.T) {
		rec := get(t, "", nil, "page.html")
		if got := rec.Header().Get("Content-Disposition"); !strings.HasPrefix(got, "attachment;") {
			t.Errorf("disposition = %q", got)
		}
		if rec.Header().Get("X-Content-Type-Options") != "nosniff" {
			t.Error("missing nosniff")
		}
	})
	t.Run("a Range is answered with the part", func(t *testing.T) {
		rec := get(t, "", map[string]string{"Range": "bytes=2-4"}, "notes.pdf")
		if rec.Code != http.StatusPartialContent || rec.Body.String() != "234" {
			t.Errorf("status = %d, body = %q", rec.Code, rec.Body)
		}
	})
}
