package apiv2

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"gorm.io/gorm"

	e "github.com/TUM-Dev/gocast/apiv2/errors"
	protobuf "github.com/TUM-Dev/gocast/apiv2/protobuf/server"
	"github.com/TUM-Dev/gocast/model"
	"github.com/TUM-Dev/gocast/tools"
	"github.com/TUM-Dev/gocast/voice-service/pb"
)

// SectionImages generates and deletes the thumbnails of a lecture's sections. The
// generation goes to a runner or else a worker, and the worker half only exists in
// api/, so it is handed in; api.SectionImages satisfies it.
type SectionImages interface {
	// Generate makes the thumbnails of sections from the recording at playlistURL,
	// which must be signed. It blocks until the job is handed off.
	Generate(streamID uint, playlistURL string, course model.Course, sections []model.VideoSection) error
	// Delete asks a worker to delete a section thumbnail's file.
	Delete(path string) error
}

// WithSectionImages lets the section endpoints have thumbnails made. Without it the
// sections are still saved, they just get no thumbnail.
func WithSectionImages(s SectionImages) Option {
	return func(a *API) { a.sectionImages = s }
}

// WithSubtitleGenerator connects requestLectureSubtitles to the voice service. Without
// it the endpoint answers 503, the voice service being an optional deployment.
func WithSubtitleGenerator(client pb.SubtitleGeneratorClient, authToken string) Option {
	return func(a *API) {
		a.subtitles = client
		a.subtitlesAuth = authToken
	}
}

// WithMassStorage sets where uploaded attachments and thumbnails are written, the
// same directory v1 writes them to.
func WithMassStorage(dir string) Option {
	return func(a *API) { a.massStorage = dir }
}

// signPlaylists is tools.SetSignedPlaylists; a variable so the tests run without the
// server's signing key.
var signPlaylists = tools.SetSignedPlaylists

var (
	errNoSuchSection = errors.New("no such section")
	errNoSuchFile    = errors.New("no such attachment")
	errNoSubtitles   = errors.New("no voice service configured")
	errNoRecording   = errors.New("the lecture has no recording")

	// A language code, which the voice service is handed as is.
	languagePattern = regexp.MustCompile(`^[a-z]{2,3}$`)
)

// lectureSection validates a section's description and start. v1 took whatever it
// was sent, minutes past 59 included, and the player then sorted them nonsensically.
func lectureSection(description string, hours, minutes, seconds uint32) (model.VideoSection, error) {
	description = strings.TrimSpace(description)
	if description == "" {
		return model.VideoSection{}, e.WithStatus(http.StatusBadRequest, errors.New("a section needs a description"))
	}
	if minutes >= 60 || seconds >= 60 {
		return model.VideoSection{}, e.WithStatus(http.StatusBadRequest, errors.New("minutes and seconds must be below 60"))
	}
	return model.VideoSection{
		Description: description,
		StartHours:  uint(hours), StartMinutes: uint(minutes), StartSeconds: uint(seconds),
	}, nil
}

func sectionAdmin(v model.VideoSection) *protobuf.LectureVideoSectionAdmin {
	return &protobuf.LectureVideoSectionAdmin{
		Id: uint32(v.ID), Description: v.Description,
		StartHours: uint32(v.StartHours), StartMinutes: uint32(v.StartMinutes), StartSeconds: uint32(v.StartSeconds),
		FileId: uint32(v.FileID),
	}
}

// lectureSectionOf loads a section and checks it is the lecture's. v1 took the section
// ID alone, so it could edit or delete any lecture's sections.
func (a *API) lectureSectionOf(stream model.Stream, sectionID uint32) (model.VideoSection, error) {
	section, err := a.dao.VideoSectionDao.Get(uint(sectionID))
	if err != nil {
		return section, e.FromGorm(err, "can't find section")
	}
	// Get uses Find, which reports a missing row as a zero value and no error.
	if section.ID == 0 || section.StreamID != stream.ID {
		return model.VideoSection{}, e.WithStatus(http.StatusNotFound, errNoSuchSection)
	}
	return section, nil
}

// generateSectionImages hands sections to SectionImages in the background, as v1 did:
// the job takes as long as fetching frames from the recording does. Failures are only
// logged, the sections themselves being saved.
//
// v1 also sent lectures without a recording, which no runner or worker can do anything
// with; those are skipped here. RegenerateThumbs makes them once there is one.
func (a *API) generateSectionImages(ctx context.Context, stream model.Stream, sections []model.VideoSection) {
	if a.sectionImages == nil || len(sections) == 0 || stream.PlaylistUrl == "" {
		return
	}
	course, err := a.dao.GetCourseById(ctx, stream.CourseID)
	if err != nil {
		a.log.Error("can't load course for section images", "stream", stream.ID, "err", err)
		return
	}
	if err := signPlaylists(&stream, nil, false); err != nil {
		a.log.Error("can't sign playlist for section images", "stream", stream.ID, "err", err)
		return
	}
	go func() {
		if err := a.sectionImages.Generate(stream.ID, stream.PlaylistUrl, course, sections); err != nil {
			a.log.Error("failed to generate video section images", "stream", stream.ID, "err", err)
		}
	}()
}

// CreateLectureSections replaces v1's createVideoSectionBatch. v1 bound the sections
// whole, stream_id included, so a request could add sections to any lecture; here
// they are the lecture's whatever is sent.
func (a *API) CreateLectureSections(ctx context.Context, req *protobuf.CreateLectureSectionsRequest) (*protobuf.CreateLectureSectionsResponse, error) {
	stream, err := a.courseLecture(ctx, req.GetCourseId(), req.GetStreamId())
	if err != nil {
		return nil, err
	}
	if len(req.GetSections()) == 0 {
		return nil, e.WithStatus(http.StatusBadRequest, errors.New("no sections given"))
	}

	sections := make([]model.VideoSection, 0, len(req.GetSections()))
	for _, in := range req.GetSections() {
		section, err := lectureSection(in.GetDescription(), in.GetStartHours(), in.GetStartMinutes(), in.GetStartSeconds())
		if err != nil {
			return nil, err
		}
		section.StreamID = stream.ID
		sections = append(sections, section)
	}

	if err := a.dao.VideoSectionDao.Create(sections); err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}
	all, err := a.dao.VideoSectionDao.GetByStreamId(stream.ID)
	if err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}

	// All of them, as v1 does: on the runner path an image is replaced in place, so
	// redoing the old ones costs time and nothing else.
	a.generateSectionImages(ctx, stream, all)

	out := make([]*protobuf.LectureVideoSectionAdmin, 0, len(all))
	for _, s := range all {
		out = append(out, sectionAdmin(s))
	}
	return &protobuf.CreateLectureSectionsResponse{Sections: out}, nil
}

// UpdateLectureSection replaces v1's updateVideoSection, which wrote through gorm's
// Updates and so silently kept the old value for anything set to zero: a section
// could not be moved to the start of the hour, nor of the lecture. v1 also kept the
// thumbnail of the old start; a moved section has its thumbnail made again.
func (a *API) UpdateLectureSection(ctx context.Context, req *protobuf.UpdateLectureSectionRequest) (*emptypb.Empty, error) {
	stream, err := a.courseLecture(ctx, req.GetCourseId(), req.GetStreamId())
	if err != nil {
		return nil, err
	}
	update, err := lectureSection(req.GetDescription(), req.GetStartHours(), req.GetStartMinutes(), req.GetStartSeconds())
	if err != nil {
		return nil, err
	}
	section, err := a.lectureSectionOf(stream, req.GetSectionId())
	if err != nil {
		return nil, err
	}

	moved := section.StartHours != update.StartHours || section.StartMinutes != update.StartMinutes ||
		section.StartSeconds != update.StartSeconds
	section.Description = update.Description
	section.StartHours, section.StartMinutes, section.StartSeconds = update.StartHours, update.StartMinutes, update.StartSeconds

	if err := a.dao.VideoSectionDao.UpdateContent(&section); err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}
	if moved {
		a.generateSectionImages(ctx, stream, []model.VideoSection{section})
	}
	return &emptypb.Empty{}, nil
}

// DeleteLectureSection replaces v1's deleteVideoSection, asking a worker to delete the
// thumbnail as it did. v1 answered 500 when the thumbnail's file row could not be
// read, after the section was already gone; that is only logged here.
func (a *API) DeleteLectureSection(ctx context.Context, req *protobuf.DeleteLectureSectionRequest) (*emptypb.Empty, error) {
	stream, err := a.courseLecture(ctx, req.GetCourseId(), req.GetStreamId())
	if err != nil {
		return nil, err
	}
	section, err := a.lectureSectionOf(stream, req.GetSectionId())
	if err != nil {
		return nil, err
	}

	if err := a.dao.VideoSectionDao.Delete(section.ID); err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}

	// v1 also asked when there was no thumbnail, handing the worker an empty path.
	if section.FileID == 0 || a.sectionImages == nil {
		return &emptypb.Empty{}, nil
	}
	file, err := a.dao.FileDao.GetFileById(fmt.Sprintf("%d", section.FileID))
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			a.log.Error("can't get video section thumbnail file", "section", section.ID, "err", err)
		}
		return &emptypb.Empty{}, nil
	}
	go func() {
		if err := a.sectionImages.Delete(file.Path); err != nil {
			a.log.Error("failed to delete video section image", "section", section.ID, "err", err)
		}
	}()
	return &emptypb.Empty{}, nil
}

// isURLPath reports whether a file row points at a URL rather than a file on disk.
// Attachments by URL were removed from v1 in #1841, but older rows remain. v1's
// File.IsURL compares the scheme to "https://", so it never matches, and deleting one
// of those answered 500 trying to remove the URL from disk.
func isURLPath(path string) bool {
	u, err := url.Parse(path)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https")
}

// removeStoredFile deletes a file row's file. One already gone is not an error: v1
// answered 500 then and kept the row, which nothing could delete any more.
func removeStoredFile(path string) error {
	if path == "" || isURLPath(path) {
		return nil
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}

// DeleteLectureAttachment replaces v1's deleteAttachment, which took the file ID
// alone: any course's administrators could delete any file, a recording or another
// lecture's thumbnail included. Here it must be one of the lecture's attachments; the
// custom thumbnail has deleteLectureThumbnail.
func (a *API) DeleteLectureAttachment(ctx context.Context, req *protobuf.DeleteLectureAttachmentRequest) (*emptypb.Empty, error) {
	stream, err := a.courseLecture(ctx, req.GetCourseId(), req.GetStreamId())
	if err != nil {
		return nil, err
	}
	file, err := a.dao.FileDao.GetFileById(fmt.Sprintf("%d", req.GetFileId()))
	if err != nil {
		return nil, e.FromGorm(err, errNoSuchFile.Error())
	}
	if file.StreamID != stream.ID || file.Type != model.FILETYPE_ATTACHMENT {
		return nil, e.WithStatus(http.StatusNotFound, errNoSuchFile)
	}

	// The file first, as v1: a failure leaves the row, so it can be tried again.
	if err := removeStoredFile(file.Path); err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}
	if err := a.dao.FileDao.DeleteFile(file.ID); err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}
	return &emptypb.Empty{}, nil
}

// DeleteLectureThumbnail deletes the custom thumbnail. v1 had no endpoint of its own:
// the old page sent the thumbnail's file ID to deleteAttachment, which this narrows to
// attachments.
//
// custom_thumbnail_enabled is left alone, as v1's upload and delete left it: nothing
// sets it, and the player shows a custom thumbnail whenever there is one.
func (a *API) DeleteLectureThumbnail(ctx context.Context, req *protobuf.DeleteLectureThumbnailRequest) (*emptypb.Empty, error) {
	stream, err := a.courseLecture(ctx, req.GetCourseId(), req.GetStreamId())
	if err != nil {
		return nil, err
	}
	thumb, err := a.dao.FileDao.GetThumbnail(stream.ID, model.FILETYPE_THUMB_CUSTOM)
	if err != nil {
		return nil, e.FromGorm(err, "the lecture has no custom thumbnail")
	}
	if err := removeStoredFile(thumb.Path); err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}
	if err := a.dao.FileDao.DeleteFile(thumb.ID); err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}
	return &emptypb.Empty{}, nil
}

// RequestLectureSubtitles replaces v1's requestSubtitles, sending the voice service the
// same request. v1 dialled it per request and answered 500 whatever went wrong; an
// unconfigured or unreachable voice service is a 503 here.
func (a *API) RequestLectureSubtitles(ctx context.Context, req *protobuf.RequestLectureSubtitlesRequest) (*emptypb.Empty, error) {
	stream, err := a.courseLecture(ctx, req.GetCourseId(), req.GetStreamId())
	if err != nil {
		return nil, err
	}
	if !languagePattern.MatchString(req.GetLanguage()) {
		return nil, e.WithStatus(http.StatusBadRequest, errors.New("language must be a language code such as de or en"))
	}
	if a.subtitles == nil {
		return nil, e.WithStatus(http.StatusServiceUnavailable, errNoSubtitles)
	}

	user, _ := a.getCurrent(ctx)
	if err := signPlaylists(&stream, user, false); err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}
	var playlist string
	switch {
	case stream.PlaylistUrl != "":
		playlist = stream.PlaylistUrl
	case stream.PlaylistUrlPRES != "":
		playlist = stream.PlaylistUrlPRES
	case stream.PlaylistUrlCAM != "":
		playlist = stream.PlaylistUrlCAM
	default:
		return nil, e.WithStatus(http.StatusBadRequest, errNoRecording)
	}

	// Only outgoing metadata is sent, so the caller's credentials stay here.
	out := ctx
	if a.subtitlesAuth != "" {
		out = metadata.AppendToOutgoingContext(out, "auth", a.subtitlesAuth)
	}
	_, err = a.subtitles.Generate(out, &pb.GenerateRequest{
		StreamId: int32(stream.ID), SourceFile: playlist, Language: req.GetLanguage(),
	})
	if err != nil {
		if status.Code(err) == codes.Unavailable {
			return nil, e.WithStatus(http.StatusServiceUnavailable, errors.New("the voice service is unreachable"))
		}
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}
	return &emptypb.Empty{}, nil
}

// transcodedVersions are the versions a lecture can be transcoding, in the order the
// card lists them.
var transcodedVersions = []model.StreamVersion{model.COMB, model.PRES, model.CAM}

// GetLectureTranscodingProgress replaces v1's getTranscodingProgress, which answered
// one version per request and 100 for a version with no progress row. The row is
// removed when a version finishes, so the absent versions are left out here, and an
// empty list means done.
//
// Read from the progress table rather than the lecture, whose cached copy can be ten
// seconds old: this is what the card polls.
func (a *API) GetLectureTranscodingProgress(ctx context.Context, req *protobuf.GetLectureTranscodingProgressRequest) (*protobuf.GetLectureTranscodingProgressResponse, error) {
	stream, err := a.courseLecture(ctx, req.GetCourseId(), req.GetStreamId())
	if err != nil {
		return nil, err
	}
	progresses := make([]*protobuf.LectureTranscodingProgress, 0, len(transcodedVersions))
	for _, v := range transcodedVersions {
		p, err := a.dao.StreamsDao.GetTranscodingProgressByVersion(v, stream.ID)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			continue
		}
		if err != nil {
			return nil, e.WithStatus(http.StatusInternalServerError, err)
		}
		progresses = append(progresses, &protobuf.LectureTranscodingProgress{Version: string(v), Progress: int32(p.Progress)})
	}
	return &protobuf.GetLectureTranscodingProgressResponse{Progresses: progresses}, nil
}
