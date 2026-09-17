package apiv2

import (
	"errors"
	"net/http"

	e "github.com/TUM-Dev/gocast/apiv2/errors"
	"github.com/TUM-Dev/gocast/model"
	"github.com/TUM-Dev/gocast/pkg/camera"
)

// CamService hands out a controller for one camera. Declared here rather than taken
// from pkg/camera so the tests can stand in a fake; *camera.Service satisfies it.
//
// The same interface exists in api/ and pkg/runner_manager for the same reason. They
// are not worth unifying while v1 is being deleted.
type CamService interface {
	For(address string, cameraType model.CameraType) (camera.Cam, error)
}

var (
	// The server was built without a camera service. Every deployment wires one, so
	// this is a programming error rather than a configuration the user can fix.
	errNoCamService = errors.New("no camera service configured")

	// A lecture hall with no camera address is a normal thing to have — a room that
	// records a stream feed and nothing else — so the caller is told, not the log.
	errNoCameraInLectureHall = errors.New("this lecture hall has no camera")

	// Snapshots are written to disk, which only makes sense with somewhere to write.
	errNoPresetImageDir = errors.New("no directory configured for preset images")
)

// WithCamService gives the API access to the lecture hall cameras. Without it the
// endpoints that reach a camera refuse; everything else works, which is what keeps
// the camera out of the tests that do not care about it.
func WithCamService(cams CamService) Option {
	return func(a *API) { a.cams = cams }
}

// WithPresetImageDir sets where a preset's snapshot is written. In production this is
// the static directory the images are then served from, so a snapshot taken here
// appears under /public.
func WithPresetImageDir(dir string) Option {
	return func(a *API) { a.presetImageDir = dir }
}

// cameraFor returns a controller for a lecture hall's camera.
//
// The three ways this fails are different kinds of problem and get different statuses:
// a missing service is the server's fault, a hall without a camera is the request's,
// and an unreachable camera is neither — it is a room whose hardware is off, which is
// worth retrying and so Unavailable rather than an error the caller can fix.
//
// The statuses are limited to the ones e.WithStatus maps; anything else it logs a
// warning for and collapses into Unknown, which would lose exactly this distinction.
func (a *API) cameraFor(lectureHall model.LectureHall) (camera.Cam, error) {
	if a.cams == nil {
		return nil, e.WithStatus(http.StatusInternalServerError, errNoCamService)
	}

	if lectureHall.CameraIP == "" {
		return nil, e.WithStatus(http.StatusBadRequest, errNoCameraInLectureHall)
	}

	cam, err := a.cams.For(lectureHall.CameraIP, lectureHall.CameraType)
	if err != nil {
		return nil, e.WithStatus(http.StatusServiceUnavailable, err)
	}

	return cam, nil
}

// snapshotDir returns where a snapshot may be written, refusing rather than writing
// to the working directory when nothing was configured.
func (a *API) snapshotDir() (string, error) {
	if a.presetImageDir == "" {
		return "", e.WithStatus(http.StatusInternalServerError, errNoPresetImageDir)
	}

	return a.presetImageDir, nil
}
