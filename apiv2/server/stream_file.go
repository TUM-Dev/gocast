package apiv2

import (
	"errors"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	e "github.com/TUM-Dev/gocast/apiv2/errors"
	"github.com/TUM-Dev/gocast/model"
)

// streamFilePath is GET /api/v2/streams/{stream_id}/files/{file_id},
// with ?download=1 to have the browser save the file rather than show it.
//
// It replaces v1's /api/download/:id (with and without ?type=serve) and
// /api/stream/:id/thumbs/:fid. The file is addressed through its stream, so the stream
// is what access is decided on; v1's download endpoint looked the file up by ID alone.
const streamFilePath = "GET /api/v2/streams/{stream_id}/files/{file_id}"

type streamFileRoute struct {
	streamID, fileID uint32
}

// GetStreamId lets the route stand in for a request in authorizeUserForStreamCourse.
func (r streamFileRoute) GetStreamId() uint32 { return r.streamID }

// streamFileRouteOf reads the route's ids. The pattern matches any text there, so a
// request whose ids are not numbers is not this endpoint's and goes to the gateway.
func streamFileRouteOf(r *http.Request) (streamFileRoute, bool) {
	streamID, err1 := strconv.ParseUint(r.PathValue("stream_id"), 10, 32)
	fileID, err2 := strconv.ParseUint(r.PathValue("file_id"), 10, 32)
	if err1 != nil || err2 != nil {
		return streamFileRoute{}, false
	}
	return streamFileRoute{streamID: uint32(streamID), fileID: uint32(fileID)}, true
}

// errNoSuchStreamFile is every refusal's message, so a refusal does not say whether
// the file exists.
var errNoSuchStreamFile = errors.New("no such file")

// inlineTypes are the content types shown in the browser when not asked to download.
// Attachments are uploaded by lecturers and served from the app's own origin, so
// anything that could run script there (html, svg) is always a download.
var inlineTypes = map[string]bool{
	"image/jpeg": true, "image/png": true, "image/gif": true, "image/webp": true,
	"application/pdf": true,
}

// handleStreamFile serves one file of a stream to whoever may have it.
//
// Who may: anyone who may watch the stream (authorizeUserForStreamCourse, so public
// courses work signed out) gets thumbnails, section images and attachments. A
// recording's file is a download, and gets v1's download rule: the course allows
// downloads and the caller is signed in, or the caller administers the course.
// v1 applied that rule to every file (so students could not open attachments of a
// course with downloads off) and none at all to ?type=serve (so any signed-in user
// could read any file by ID).
//
// A caller who may not have the file is answered 404 as if it did not exist, except
// that a signed-out caller is answered 401 when signing in could change the answer.
func (a *API) handleStreamFile(w http.ResponseWriter, r *http.Request, route streamFileRoute) {
	ctx := incomingContext(r)

	user, stream, course, err := a.authorizeUserForStreamCourse(ctx, route)
	if err != nil {
		if status.Code(err) == codes.PermissionDenied {
			// Signed in, or signed out where no login would help: nothing to see.
			if u, cerr := a.currentOrAnonymous(ctx); cerr == nil && u == nil && (course.IsLoggedIn() || course.IsEnrolled()) && !stream.Private {
				writeUploadError(w, e.WithStatus(http.StatusUnauthorized, errors.New("sign in to access this file")))
				return
			}
			err = e.WithStatus(http.StatusNotFound, errNoSuchStreamFile)
		}
		writeUploadError(w, err)
		return
	}

	file, err := a.dao.FileDao.GetFileById(strconv.FormatUint(uint64(route.fileID), 10))
	if err != nil || file.ID == 0 || file.StreamID != stream.ID {
		writeUploadError(w, e.WithStatus(http.StatusNotFound, errNoSuchStreamFile))
		return
	}

	switch file.Type {
	case model.FILETYPE_VOD:
		isAdmin := user.IsAdminOfCourse(course)
		if !stream.IsDownloadable() || !(isAdmin || (course.DownloadsEnabled && user != nil)) {
			if user == nil && course.DownloadsEnabled && stream.IsDownloadable() {
				writeUploadError(w, e.WithStatus(http.StatusUnauthorized, errors.New("sign in to download this file")))
				return
			}
			writeUploadError(w, e.WithStatus(http.StatusNotFound, errNoSuchStreamFile))
			return
		}
	case model.FILETYPE_ATTACHMENT, model.FILETYPE_IMAGE_JPG,
		model.FILETYPE_THUMB_COMB, model.FILETYPE_THUMB_CAM, model.FILETYPE_THUMB_PRES,
		model.FILETYPE_THUMB_LG_COMB, model.FILETYPE_THUMB_LG_CAM, model.FILETYPE_THUMB_LG_PRES,
		model.FILETYPE_THUMB_LG_CAM_PRES, model.FILETYPE_THUMB_CUSTOM:
		// Whoever may watch the stream: decided above.
	default:
		writeUploadError(w, e.WithStatus(http.StatusNotFound, errNoSuchStreamFile))
		return
	}

	a.sendStreamFile(w, r, file, r.URL.Query().Get("download") == "1")
}

// sendStreamFile streams the file from disk, with Range support, rather than reading
// it into memory: recordings are gigabytes.
func (a *API) sendStreamFile(w http.ResponseWriter, r *http.Request, file model.File, download bool) {
	f, err := os.Open(file.Path)
	if err != nil {
		writeUploadError(w, e.WithStatus(http.StatusNotFound, errNoSuchStreamFile))
		return
	}
	defer func() { _ = f.Close() }()
	info, err := f.Stat()
	if err != nil || info.IsDir() {
		writeUploadError(w, e.WithStatus(http.StatusNotFound, errNoSuchStreamFile))
		return
	}

	name := file.Filename
	if name == "" {
		name = file.GetDownloadFileName()
	}
	contentType := mime.TypeByExtension(strings.ToLower(filepath.Ext(name)))
	if contentType == "" {
		contentType = mime.TypeByExtension(strings.ToLower(filepath.Ext(file.Path)))
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	mediaType, _, _ := mime.ParseMediaType(contentType)

	disposition := "attachment"
	if !download && inlineTypes[mediaType] {
		disposition = "inline"
	}
	// FormatMediaType quotes and escapes the name, which a lecturer chose; it fails
	// only for a name that cannot go in a header, which gets a generic one.
	cd := mime.FormatMediaType(disposition, map[string]string{"filename": name})
	if cd == "" {
		cd = disposition
	}

	h := w.Header()
	h.Set("Content-Type", contentType)
	h.Set("Content-Disposition", cd)
	// A file is served from the app's own origin: never let it be sniffed into
	// something else, nor run script if opened.
	h.Set("X-Content-Type-Options", "nosniff")
	if mediaType != "application/pdf" {
		// The pdf viewer does not run in a sandbox.
		h.Set("Content-Security-Policy", "sandbox")
	}
	// Access depends on the cookie, so no shared cache may keep it.
	h.Set("Cache-Control", "private, max-age=0, must-revalidate")

	a.log.Info("file request", "file", file.ID, "stream", file.StreamID, "download", download)
	// ServeContent answers Range, If-Range and If-Modified-Since from the reader.
	http.ServeContent(w, r, name, info.ModTime().Truncate(time.Second), f)
}
