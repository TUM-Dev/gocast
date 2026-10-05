package apiv2

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"go.uber.org/mock/gomock"
	"gorm.io/gorm"

	"github.com/TUM-Dev/gocast/model"
)

func TestLectureUploadRouteOf(t *testing.T) {
	for path, want := range map[string]*lectureUploadRoute{
		"/api/v2/courses/1/streams/7/attachments":   {courseID: 1, streamID: 7, kind: uploadAttachment},
		"/api/v2/courses/1/streams/7/thumbnail":     {courseID: 1, streamID: 7, kind: uploadThumbnail},
		"/api/v2/courses/1/streams/7/sections":      nil,
		"/api/v2/courses/1/streams/7/attachments/5": nil,
		"/api/v2/courses/x/streams/7/attachments":   nil,
		"/api/v2/courses/1/lectures/7/thumbnail":    nil,
		"/api/v2/courses/1/streams/7":               nil,
	} {
		got, ok := lectureUploadRouteOf(httptest.NewRequest(http.MethodPost, path, nil))
		if (want == nil) == ok || (want != nil && got != *want) {
			t.Errorf("%s: got %+v, %v; want %+v", path, got, ok, want)
		}
	}
	if _, ok := lectureUploadRouteOf(httptest.NewRequest(http.MethodDelete, "/api/v2/courses/1/streams/7/thumbnail", nil)); ok {
		t.Error("DELETE of the thumbnail is the RPC's")
	}
}

// uploadRequest is a multipart POST of one file, made by user (nil for nobody). The
// caller rides in the context the way the interceptor leaves it for an RPC, which
// getCurrent reads before looking at any credentials.
func uploadRequest(t *testing.T, path string, user *model.User, filename, contentType string) *http.Request {
	t.Helper()
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	h := textproto.MIMEHeader{}
	h.Set("Content-Disposition", `form-data; name="file"; filename="`+filename+`"`)
	h.Set("Content-Type", contentType)
	part, err := w.CreatePart(h)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = part.Write([]byte("content"))
	_ = w.Close()

	req := httptest.NewRequest(http.MethodPost, path, &body)
	req.Header.Set("Content-Type", w.FormDataContentType())
	if user != nil {
		req = req.WithContext(asCaller(user))
	}
	return req
}

func serveUpload(api *API, req *http.Request) *httptest.ResponseRecorder {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Any("/api/v2/*any", func(c *gin.Context) {
		route, ok := lectureUploadRouteOf(c.Request)
		if !ok {
			c.Status(http.StatusTeapot)
			return
		}
		api.handleLectureUpload(c, route)
	})
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

// filesUnder lists every file below dir.
func filesUnder(t *testing.T, dir string) []string {
	t.Helper()
	var out []string
	_ = filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			out = append(out, p)
		}
		return nil
	})
	return out
}

func TestUploadLectureAttachment(t *testing.T) {
	const path = "/api/v2/courses/1/streams/7/attachments"

	t.Run("stores the file where v1 does and answers it", func(t *testing.T) {
		api, m := newContentAPI(t)
		api.massStorage = t.TempDir()
		m.courses.EXPECT().GetCourseById(gomock.Any(), uint(1)).Return(courseOne, nil).Times(2)
		m.streams.EXPECT().GetStreamByID(gomock.Any(), "7").Return(courseOneLecture(), nil)
		var saved model.File
		m.files.EXPECT().NewFile(gomock.Any()).DoAndReturn(func(f *model.File) error {
			f.ID = 42
			saved = *f
			return nil
		})

		rec := serveUpload(api, uploadRequest(t, path, lectureAdmin, "slides.pdf", "application/pdf"))
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d: %s", rec.Code, rec.Body)
		}
		var got struct {
			ID           int    `json:"id"`
			Type         int    `json:"type"`
			FriendlyName string `json:"friendlyName"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
			t.Fatalf("body %s: %v", rec.Body, err)
		}
		if got.ID != 42 || got.Type != model.FILETYPE_ATTACHMENT || got.FriendlyName != "slides.pdf" {
			t.Errorf("answered %+v", got)
		}

		wantDir := filepath.Join(api.massStorage, "Course.2026", "Course.W", "files")
		if filepath.Dir(saved.Path) != wantDir || filepath.Ext(saved.Path) != ".pdf" || saved.StreamID != 7 || saved.Filename != "slides.pdf" {
			t.Errorf("saved %+v, want it in %s", saved, wantDir)
		}
		content, err := os.ReadFile(saved.Path)
		if err != nil || string(content) != "content" {
			t.Errorf("file on disk: %q, %v", content, err)
		}
	})

	t.Run("401s an anonymous request", func(t *testing.T) {
		api, _ := newContentAPI(t)
		api.massStorage = t.TempDir()
		rec := serveUpload(api, uploadRequest(t, path, nil, "slides.pdf", "application/pdf"))
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("status = %d: %s", rec.Code, rec.Body)
		}
		if files := filesUnder(t, api.massStorage); len(files) != 0 {
			t.Errorf("wrote %v", files)
		}
	})

	// 404 rather than 403, as the RPCs answer through authorizeCourseAdmin: the status
	// must not tell someone else's course from a missing one.
	t.Run("404s a caller who does not administer the course", func(t *testing.T) {
		api, m := newContentAPI(t)
		api.massStorage = t.TempDir()
		student := &model.User{Model: gorm.Model{ID: 9}, Role: model.StudentType}
		m.courses.EXPECT().GetCourseById(gomock.Any(), uint(1)).Return(courseOne, nil)

		rec := serveUpload(api, uploadRequest(t, path, student, "slides.pdf", "application/pdf"))
		if rec.Code != http.StatusNotFound {
			t.Errorf("status = %d: %s", rec.Code, rec.Body)
		}
		if files := filesUnder(t, api.massStorage); len(files) != 0 {
			t.Errorf("wrote %v", files)
		}
	})

	t.Run("404s another course's lecture and writes nothing", func(t *testing.T) {
		api, m := newContentAPI(t)
		api.massStorage = t.TempDir()
		m.courses.EXPECT().GetCourseById(gomock.Any(), uint(1)).Return(courseOne, nil)
		m.streams.EXPECT().GetStreamByID(gomock.Any(), "7").Return(foreignLecture(), nil)

		rec := serveUpload(api, uploadRequest(t, path, lectureAdmin, "slides.pdf", "application/pdf"))
		if rec.Code != http.StatusNotFound {
			t.Errorf("status = %d: %s", rec.Code, rec.Body)
		}
		if files := filesUnder(t, api.massStorage); len(files) != 0 {
			t.Errorf("wrote %v", files)
		}
	})

	t.Run("400s a form without a file", func(t *testing.T) {
		api, m := newContentAPI(t)
		api.massStorage = t.TempDir()
		m.courses.EXPECT().GetCourseById(gomock.Any(), uint(1)).Return(courseOne, nil)
		m.streams.EXPECT().GetStreamByID(gomock.Any(), "7").Return(courseOneLecture(), nil)

		req := httptest.NewRequest(http.MethodPost, path, bytes.NewBufferString("--x--\r\n"))
		req.Header.Set("Content-Type", "multipart/form-data; boundary=x")
		rec := serveUpload(api, req.WithContext(asCaller(lectureAdmin)))
		if rec.Code != http.StatusBadRequest {
			t.Errorf("status = %d: %s", rec.Code, rec.Body)
		}
	})

	t.Run("removes the file when the row cannot be written", func(t *testing.T) {
		api, m := newContentAPI(t)
		api.massStorage = t.TempDir()
		m.courses.EXPECT().GetCourseById(gomock.Any(), uint(1)).Return(courseOne, nil).Times(2)
		m.streams.EXPECT().GetStreamByID(gomock.Any(), "7").Return(courseOneLecture(), nil)
		m.files.EXPECT().NewFile(gomock.Any()).Return(gorm.ErrInvalidDB)

		rec := serveUpload(api, uploadRequest(t, path, lectureAdmin, "slides.pdf", "application/pdf"))
		if rec.Code != http.StatusInternalServerError {
			t.Errorf("status = %d: %s", rec.Code, rec.Body)
		}
		if files := filesUnder(t, api.massStorage); len(files) != 0 {
			t.Errorf("left %v", files)
		}
	})
}

func TestUploadLectureThumbnail(t *testing.T) {
	const path = "/api/v2/courses/1/streams/7/thumbnail"

	t.Run("replaces the custom thumbnail, its old file included", func(t *testing.T) {
		api, m := newContentAPI(t)
		api.massStorage = t.TempDir()
		old := writeTempFile(t)
		m.courses.EXPECT().GetCourseById(gomock.Any(), uint(1)).Return(courseOne, nil).Times(2)
		m.streams.EXPECT().GetStreamByID(gomock.Any(), "7").Return(courseOneLecture(), nil)
		gomock.InOrder(
			m.files.EXPECT().GetThumbnail(uint(7), model.FileType(model.FILETYPE_THUMB_CUSTOM)).
				Return(model.File{Model: gorm.Model{ID: 3}, Path: old}, nil),
			m.files.EXPECT().SetThumbnail(uint(7), gomock.Any()).DoAndReturn(func(_ uint, f model.File) error {
				wantDir := filepath.Join(api.massStorage, "Course.2026.W", "files")
				if f.Type != model.FILETYPE_THUMB_CUSTOM || filepath.Dir(f.Path) != wantDir || f.CourseName != "Course" {
					t.Errorf("set %+v, want it in %s", f, wantDir)
				}
				return nil
			}),
			m.files.EXPECT().GetThumbnail(uint(7), model.FileType(model.FILETYPE_THUMB_CUSTOM)).
				Return(model.File{Model: gorm.Model{ID: 4}, Type: model.FILETYPE_THUMB_CUSTOM, Filename: "cover.png"}, nil),
		)

		rec := serveUpload(api, uploadRequest(t, path, lectureAdmin, "cover.png", "image/png"))
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d: %s", rec.Code, rec.Body)
		}
		var got struct {
			ID int `json:"id"`
		}
		_ = json.Unmarshal(rec.Body.Bytes(), &got)
		if got.ID != 4 {
			t.Errorf("answered %s, want the new thumbnail's id", rec.Body)
		}
		if _, err := os.Stat(old); !os.IsNotExist(err) {
			t.Errorf("old thumbnail still there: %v", err)
		}
		if files := filesUnder(t, api.massStorage); len(files) != 1 {
			t.Errorf("stored %v", files)
		}
	})

	for name, upload := range map[string][2]string{
		"a file that is not an image": {"cover.png", "application/pdf"},
		"an image format v1 refuses":  {"cover.tiff", "image/tiff"},
	} {
		t.Run("400s "+name+" and writes nothing", func(t *testing.T) {
			api, m := newContentAPI(t)
			api.massStorage = t.TempDir()
			m.courses.EXPECT().GetCourseById(gomock.Any(), uint(1)).Return(courseOne, nil).Times(2)
			m.streams.EXPECT().GetStreamByID(gomock.Any(), "7").Return(courseOneLecture(), nil)

			rec := serveUpload(api, uploadRequest(t, path, lectureAdmin, upload[0], upload[1]))
			if rec.Code != http.StatusBadRequest {
				t.Errorf("status = %d: %s", rec.Code, rec.Body)
			}
			if files := filesUnder(t, api.massStorage); len(files) != 0 {
				t.Errorf("wrote %v", files)
			}
		})
	}

	t.Run("401s an anonymous request", func(t *testing.T) {
		api, _ := newContentAPI(t)
		rec := serveUpload(api, uploadRequest(t, path, nil, "cover.png", "image/png"))
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("status = %d: %s", rec.Code, rec.Body)
		}
	})
}
