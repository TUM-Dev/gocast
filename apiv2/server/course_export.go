package apiv2

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	e "github.com/TUM-Dev/gocast/apiv2/errors"
	protobuf "github.com/TUM-Dev/gocast/apiv2/protobuf/server"
	"github.com/TUM-Dev/gocast/model"
	"github.com/TUM-Dev/gocast/tools"
)

// ExportCourseVideos answers with a signed download link for every recorded lecture of the
// course, in one version or, by default, in every version it has. It hands out links rather than the files: the edge remuxes each
// download into a temp file, so the client fetching them one after another keeps the load
// on the edge bounded and lets a failed or interrupted file be retried on its own.
func (a *API) ExportCourseVideos(ctx context.Context, req *protobuf.ExportCourseVideosRequest) (*protobuf.ExportCourseVideosResponse, error) {
	course, err := a.administeredCourse(ctx, req.GetCourseId())
	if err != nil {
		return nil, err
	}
	user, err := a.getCurrent(ctx)
	if err != nil {
		return nil, err
	}
	versions, err := exportVersions(req.GetKind())
	if err != nil {
		return nil, err
	}

	streams, err := a.dao.StreamsDao.GetRecordedStreamsForExport(ctx, course.ID)
	if err != nil {
		return nil, e.FromGorm(err, "can't load the lectures")
	}

	used := make(map[string]bool)
	videos := make([]*protobuf.CourseVideoExport, 0, len(streams))
	for _, stream := range streams {
		// Playlists the signer skips (lrz.de) come back unchanged, without a token the edge
		// would accept, so a playlist that differs from the unsigned one is a signed one.
		unsigned := make(map[model.StreamVersion]string, len(versions))
		for _, v := range versions {
			unsigned[v] = exportPlaylist(stream, v)
		}
		// Signing mutates the playlist fields of the copy, all versions at once.
		if err := tools.SetSignedPlaylistsForCourse(&stream, user, course.Slug, true); err != nil {
			a.log.Error("can't sign playlist for export", "stream", stream.ID, "err", err)
			return nil, e.WithStatus(http.StatusInternalServerError, errors.New("could not sign the download links"))
		}
		for _, v := range versions {
			playlist := exportPlaylist(stream, v)
			if playlist == unsigned[v] {
				continue
			}
			videos = append(videos, &protobuf.CourseVideoExport{
				StreamId: uint32(stream.ID),
				FileName: uniqueExportName(used, tools.DownloadFileName(course.Slug, &stream, string(v)), stream.ID),
				Url:      playlist + "&download=1",
			})
		}
	}

	a.audit(ctx, model.AuditContent,
		fmt.Sprintf("%s:'%s' video export requested (%d videos)", course.Name, course.Slug, len(videos)), forCourse(course.ID))
	return &protobuf.ExportCourseVideosResponse{Videos: videos}, nil
}

// exportVersions maps the requested kind to the versions to export. Unspecified means
// every version a lecture has, so an export is complete unless the caller narrows it.
func exportVersions(kind protobuf.WatchViewKind) ([]model.StreamVersion, error) {
	switch kind {
	case protobuf.WatchViewKind_WATCH_VIEW_KIND_UNSPECIFIED:
		return []model.StreamVersion{model.COMB, model.PRES, model.CAM}, nil
	case protobuf.WatchViewKind_WATCH_VIEW_KIND_COMBINED:
		return []model.StreamVersion{model.COMB}, nil
	case protobuf.WatchViewKind_WATCH_VIEW_KIND_PRESENTATION:
		return []model.StreamVersion{model.PRES}, nil
	case protobuf.WatchViewKind_WATCH_VIEW_KIND_CAMERA:
		return []model.StreamVersion{model.CAM}, nil
	}
	return nil, e.WithStatus(http.StatusBadRequest, errors.New("unknown video kind"))
}

// exportPlaylist picks the (signed) playlist of a stream for one version.
func exportPlaylist(s model.Stream, v model.StreamVersion) string {
	switch v {
	case model.CAM:
		return s.PlaylistUrlCAM
	case model.PRES:
		return s.PlaylistUrlPRES
	}
	return s.PlaylistUrl
}

// uniqueExportName keeps two lectures from saving over each other: DownloadFileName is
// built from the date and name, so unnamed lectures on one day collide. It also falls back
// to the stream id when there is nothing to name the file after.
func uniqueExportName(used map[string]bool, name string, id uint) string {
	if name == "" {
		name = fmt.Sprintf("%d.mp4", id)
	}
	// The suffixed name can collide too (a lecture literally named "7"), so keep going.
	for base, n := strings.TrimSuffix(name, ".mp4"), 0; used[name]; n++ {
		if n == 0 {
			name = fmt.Sprintf("%s_%d.mp4", base, id)
		} else {
			name = fmt.Sprintf("%s_%d_%d.mp4", base, id, n)
		}
	}
	used[name] = true
	return name
}
