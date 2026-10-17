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

// Thumbnails regenerates one VOD's large thumbnail. Only a runner understands the
// versions PlaylistUrl, PlaylistUrlCAM and PlaylistUrlPRES stand for, so the worker
// half that SectionImages has has no equivalent here. See WithThumbnails.
type Thumbnails interface {
	// Regenerate asks a runner to redo the thumbnail for one version of one VOD.
	// playlistURL must be signed.
	Regenerate(streamID uint, version model.StreamVersion, playlistURL string) error
}

// WithThumbnails lets RegenerateCourseThumbnails reach a runner. Without it the RPC
// answers 503.
func WithThumbnails(t Thumbnails) Option {
	return func(a *API) { a.thumbnails = t }
}

// RegenerateCourseThumbnails starts a background job that asks a runner to redo the
// thumbnail for every available version (combined, camera, presentation) of every
// recorded lecture in the course. It returns as soon as the jobs are queued, the way
// GenerateMaintenanceThumbnails does for every course at once.
func (a *API) RegenerateCourseThumbnails(ctx context.Context, req *protobuf.RegenerateCourseThumbnailsRequest) (*emptypb.Empty, error) {
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
				a.log.Error("can't sign playlists for thumbnail regeneration", "stream", stream.ID, "err", err)
				continue
			}
			for _, v := range []struct {
				url     string
				version model.StreamVersion
			}{
				{stream.PlaylistUrl, model.COMB},
				{stream.PlaylistUrlCAM, model.CAM},
				{stream.PlaylistUrlPRES, model.PRES},
			} {
				if v.url == "" {
					continue
				}
				if err := thumbnails.Regenerate(stream.ID, v.version, v.url); err != nil {
					a.log.Error("can't request thumbnail regeneration",
						"stream", stream.ID, "version", v.version, "err", err)
				}
			}
		}
	}()

	return &emptypb.Empty{}, nil
}
