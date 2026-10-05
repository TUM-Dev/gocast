package apiv2

import (
	"context"
	"errors"
	"fmt"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	uuid "github.com/satori/go.uuid"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"

	e "github.com/TUM-Dev/gocast/apiv2/errors"
	protobuf "github.com/TUM-Dev/gocast/apiv2/protobuf/server"
	"github.com/TUM-Dev/gocast/model"
	"github.com/TUM-Dev/gocast/tools/safepath"
)

// maxLectureUploadSize is v1's MAX_FILE_SIZE, for attachments and thumbnails alike.
const maxLectureUploadSize = 50 * 1000 * 1000

// The uploads a lecture takes. The media upload is a recording, which is not stored
// here but proxied to a worker: see lecture_media.go.
const (
	uploadAttachment = "attachments"
	uploadThumbnail  = "thumbnail"
	uploadMedia      = "media"
)

// lectureUploadRoute is a request for one of the upload endpoints:
// POST /api/v2/courses/{course_id}/streams/{stream_id}/{attachments|thumbnail|media}.
type lectureUploadRoute struct {
	courseID, streamID uint32
	kind               string
}

// GetCourseId lets the route stand in for a request in authorizeCourseAdmin.
func (r lectureUploadRoute) GetCourseId() uint32 { return r.courseID }

// lectureUploadRouteOf matches a request against the upload endpoints. Only POST: the
// thumbnail's DELETE on the same path is an RPC.
func lectureUploadRouteOf(r *http.Request) (lectureUploadRoute, bool) {
	if r.Method != http.MethodPost {
		return lectureUploadRoute{}, false
	}
	rest, ok := strings.CutPrefix(r.URL.Path, "/api/v2/courses/")
	if !ok {
		return lectureUploadRoute{}, false
	}
	parts := strings.Split(rest, "/")
	if len(parts) != 4 || parts[1] != "streams" || (parts[3] != uploadAttachment && parts[3] != uploadThumbnail && parts[3] != uploadMedia) {
		return lectureUploadRoute{}, false
	}
	courseID, err1 := strconv.ParseUint(parts[0], 10, 32)
	streamID, err2 := strconv.ParseUint(parts[2], 10, 32)
	if err1 != nil || err2 != nil {
		return lectureUploadRoute{}, false
	}
	return lectureUploadRoute{courseID: uint32(courseID), streamID: uint32(streamID), kind: parts[3]}, true
}

// incomingContext presents an HTTP request's credentials the way the gateway hands
// them to an RPC, so getCurrent authenticates both alike: the bearer token, else the
// session cookie.
func incomingContext(r *http.Request) context.Context {
	md := metadata.MD{}
	if v := r.Header.Values("Authorization"); len(v) > 0 {
		md["authorization"] = v
	}
	if v := r.Header.Values("Cookie"); len(v) > 0 {
		md["grpcgateway-cookie"] = v
	}
	return metadata.NewIncomingContext(r.Context(), md)
}

// writeUploadError answers as the gateway answers an RPC's error, so a client handles
// the uploads' failures as it does every other endpoint's.
func writeUploadError(c *gin.Context, err error) {
	st := status.Convert(err)
	c.AbortWithStatusJSON(runtime.HTTPStatusFromCode(st.Code()), gin.H{"code": int(st.Code()), "message": st.Message()})
}

// handleLectureUpload replaces v1's newAttachment and putCustomLiveThumbnail, and
// through proxyLectureMedia its uploadVODMedia. It is authorized as the lecture
// administration RPCs are: authenticated by getCurrent, the course by
// authorizeCourseAdmin, the lecture by courseLecture. A caller who does not
// administer the course is answered 404, as the RPCs answer, so the status cannot tell
// a missing course from someone else's. All of that before the body is read, so a
// refused upload is not buffered first.
func (a *API) handleLectureUpload(c *gin.Context, route lectureUploadRoute) {
	ctx := incomingContext(c.Request)
	stream, err := a.authorizeLectureUpload(ctx, route)
	if err != nil {
		writeUploadError(c, err)
		return
	}
	if route.kind == uploadMedia {
		a.proxyLectureMedia(c, stream)
		return
	}
	if a.massStorage == "" {
		writeUploadError(c, e.WithStatus(http.StatusInternalServerError, errors.New("no directory configured for uploads")))
		return
	}

	// v1 checked the size once the whole body was in. The slack is the multipart
	// framing around a file of exactly the limit.
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxLectureUploadSize+1<<20)
	upload, err := c.FormFile("file")
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			writeUploadError(c, e.WithStatus(http.StatusBadRequest, errors.New("file too large (limit is 50 MB)")))
			return
		}
		writeUploadError(c, e.WithStatus(http.StatusBadRequest, errors.New("missing form field 'file'")))
		return
	}
	if upload.Size > maxLectureUploadSize {
		writeUploadError(c, e.WithStatus(http.StatusBadRequest, errors.New("file too large (limit is 50 MB)")))
		return
	}

	course, err := a.dao.GetCourseById(ctx, stream.CourseID)
	if err != nil {
		writeUploadError(c, e.FromGorm(err, "can't find course"))
		return
	}

	var file model.File
	if route.kind == uploadAttachment {
		file, err = a.saveAttachment(c, stream, course, upload)
	} else {
		file, err = a.saveThumbnail(c, stream, course, upload)
	}
	if err != nil {
		writeUploadError(c, err)
		return
	}

	body, err := protojson.MarshalOptions{EmitUnpopulated: true}.Marshal(&protobuf.LectureFileAdmin{
		Id: uint32(file.ID), Type: uint32(file.Type), FriendlyName: file.GetFriendlyFileName(),
	})
	if err != nil {
		writeUploadError(c, e.WithStatus(http.StatusInternalServerError, err))
		return
	}
	c.Data(http.StatusOK, "application/json", body)
}

// authorizeLectureUpload answers the lecture an upload is for, once the caller is
// known to administer its course.
func (a *API) authorizeLectureUpload(ctx context.Context, route lectureUploadRoute) (model.Stream, error) {
	user, err := a.getCurrent(ctx)
	if err != nil {
		return model.Stream{}, e.WithStatus(http.StatusUnauthorized, err)
	}
	method := "POST /courses/{course_id}/streams/{stream_id}/" + route.kind
	if err := a.authorizeCourseAdmin(ctx, user, route, method); err != nil {
		return model.Stream{}, err
	}
	return a.courseLecture(ctx, route.courseID, route.streamID)
}

// storeUpload writes an upload into dir under a fresh name, keeping its extension as
// v1 does. The directory is under mass storage, built by safepath from the course's
// name, which is free text.
func storeUpload(c *gin.Context, root string, upload *multipart.FileHeader, dirs ...string) (string, error) {
	dir, err := safepath.JoinInRoot(root, dirs...)
	if err != nil {
		return "", e.WithStatus(http.StatusInternalServerError, err)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", e.WithStatus(http.StatusInternalServerError, fmt.Errorf("can't create folder: %w", err))
	}
	path := filepath.Join(dir, fmt.Sprintf("%s%s", uuid.NewV1(), filepath.Ext(upload.Filename)))
	if err := c.SaveUploadedFile(upload, path); err != nil {
		return "", e.WithStatus(http.StatusInternalServerError, fmt.Errorf("can't save file: %w", err))
	}
	return path, nil
}

// saveAttachment stores an attachment where v1's newAttachment does.
func (a *API) saveAttachment(c *gin.Context, stream model.Stream, course model.Course, upload *multipart.FileHeader) (model.File, error) {
	path, err := storeUpload(c, a.massStorage, upload,
		fmt.Sprintf("%s.%d", course.Name, course.Year),
		fmt.Sprintf("%s.%s", course.Name, course.TeachingTerm),
		"files")
	if err != nil {
		return model.File{}, err
	}

	file := model.File{StreamID: stream.ID, Path: path, Filename: upload.Filename, Type: model.FILETYPE_ATTACHMENT}
	if err := a.dao.FileDao.NewFile(&file); err != nil {
		// v1 left the file behind with nothing pointing at it.
		_ = os.Remove(path)
		return model.File{}, e.WithStatus(http.StatusInternalServerError, err)
	}
	return file, nil
}

// thumbnailExtensions are the image formats v1 accepts as a custom thumbnail.
var thumbnailExtensions = map[string]bool{".jpg": true, ".jpeg": true, ".png": true, ".gif": true, ".webp": true}

// saveThumbnail stores a custom thumbnail where v1's putCustomLiveThumbnail does, with
// the same checks, replacing any previous one. v1 left the previous file on disk and
// answered the new thumbnail's ID as 0, SetThumbnail taking its argument by value.
func (a *API) saveThumbnail(c *gin.Context, stream model.Stream, course model.Course, upload *multipart.FileHeader) (model.File, error) {
	if !strings.HasPrefix(upload.Header.Get("Content-Type"), "image/") {
		return model.File{}, e.WithStatus(http.StatusBadRequest, errors.New("the thumbnail must be an image"))
	}
	if !thumbnailExtensions[strings.ToLower(filepath.Ext(upload.Filename))] {
		return model.File{}, e.WithStatus(http.StatusBadRequest, errors.New("unsupported image format: use jpg, png, gif or webp"))
	}

	previous, previousErr := a.dao.FileDao.GetThumbnail(stream.ID, model.FILETYPE_THUMB_CUSTOM)

	path, err := storeUpload(c, a.massStorage, upload,
		fmt.Sprintf("%s.%d.%s", course.Name, course.Year, course.TeachingTerm),
		"files")
	if err != nil {
		return model.File{}, err
	}

	thumb := model.File{
		StreamID: stream.ID, Path: path, Filename: upload.Filename,
		Type: model.FILETYPE_THUMB_CUSTOM, CourseName: course.Name,
	}
	if err := a.dao.FileDao.SetThumbnail(stream.ID, thumb); err != nil {
		_ = os.Remove(path)
		return model.File{}, e.WithStatus(http.StatusInternalServerError, err)
	}
	if previousErr == nil && previous.Path != path {
		if err := removeStoredFile(previous.Path); err != nil {
			a.log.Warn("can't delete replaced custom thumbnail", "path", previous.Path, "err", err)
		}
	}

	saved, err := a.dao.FileDao.GetThumbnail(stream.ID, model.FILETYPE_THUMB_CUSTOM)
	if err != nil {
		return model.File{}, e.WithStatus(http.StatusInternalServerError, err)
	}
	return saved, nil
}
