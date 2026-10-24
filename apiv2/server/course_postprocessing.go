package apiv2

import (
	"context"
	"errors"
	"net/http"

	"google.golang.org/protobuf/types/known/emptypb"

	e "github.com/TUM-Dev/gocast/apiv2/errors"
	protobuf "github.com/TUM-Dev/gocast/apiv2/protobuf/server"
	"github.com/TUM-Dev/gocast/model"
	"github.com/TUM-Dev/gocast/tools"
)

// Thumbnails redoes the post-processing of one VOD: its large thumbnail, and the silences. Only a runner understands the
// versions PlaylistUrl, PlaylistUrlCAM and PlaylistUrlPRES stand for, so the worker
// half that SectionImages has has no equivalent here. See WithThumbnails.
type Thumbnails interface {
	// Regenerate asks a runner to redo the thumbnail for one version of one VOD, and the
	// silences of the stream if detectSilence is set. playlistURL must be signed.
	Regenerate(streamID uint, version model.StreamVersion, playlistURL string, detectSilence bool) error
}

// WithThumbnails lets RedoCoursePostProcessing reach a runner. Without it the RPC
// answers 503.
func WithThumbnails(t Thumbnails) Option {
	return func(a *API) { a.thumbnails = t }
}

// RedoCoursePostProcessing starts a background job that asks a runner to redo the
// thumbnail for every available version (combined, camera, presentation) of every
// recorded lecture in the course, and to detect the lecture's silences. It returns as
// soon as the jobs are queued, the way GenerateMaintenanceThumbnails does for every
// course at once.
func (a *API) RedoCoursePostProcessing(ctx context.Context, req *protobuf.RedoCoursePostProcessingRequest) (*emptypb.Empty, error) {
	course, err := a.administeredCourse(ctx, req.GetCourseId())
	if err != nil {
		return nil, err
	}
	if a.thumbnails == nil {
		return nil, e.WithStatus(http.StatusServiceUnavailable, errors.New("no runner configured"))
	}

	streams := course.Streams
	thumbnails := a.thumbnails
	go func() {
		for _, stream := range streams {
			// Signing mutates the playlist fields in place, so each stream gets its
			// own copy to sign rather than aliasing the loop variable.
			if err := tools.SetSignedPlaylists(&stream, &model.User{}, false); err != nil {
				a.log.Error("can't sign playlists for post-processing", "stream", stream.ID, "err", err)
				continue
			}
			// All versions of a lecture share their audio, so only one is scanned for
			// silences: the best, or the next if the runner could not be asked about it.
			scan := true
			for _, v := range postProcessingVersions(stream) {
				if err := thumbnails.Regenerate(stream.ID, v.version, v.url, scan); err != nil {
					a.log.Error("can't request post-processing",
						"stream", stream.ID, "version", v.version, "err", err)
					continue
				}
				scan = false
			}
		}
	}()

	return &emptypb.Empty{}, nil
}

type postProcessingVersion struct {
	url     string
	version model.StreamVersion
}

// postProcessingVersions lists the versions of a (signed) stream that have a VOD, best
// first: combined, camera, presentation. That is also the order silence detection
// prefers, as when the recording is made.
func postProcessingVersions(stream model.Stream) []postProcessingVersion {
	var out []postProcessingVersion
	for _, v := range []postProcessingVersion{
		{stream.PlaylistUrl, model.COMB},
		{stream.PlaylistUrlCAM, model.CAM},
		{stream.PlaylistUrlPRES, model.PRES},
	} {
		if v.url != "" {
			out = append(out, v)
		}
	}
	return out
}
