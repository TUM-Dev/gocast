package apiv2

import (
	"errors"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/TUM-Dev/gocast/model"
	"github.com/TUM-Dev/gocast/pkg/camera"
)

// fakeCam records what it was asked to do and answers however the test needs.
type fakeCam struct {
	presets      []model.CameraPreset
	presetsErr   error
	setPresetIDs []int
	snapshotDirs []string
}

func (c *fakeCam) SetPreset(presetID int) error {
	c.setPresetIDs = append(c.setPresetIDs, presetID)
	return nil
}

func (c *fakeCam) TakeSnapshot(outDir string) (string, error) {
	c.snapshotDirs = append(c.snapshotDirs, outDir)
	return "snapshot.jpg", nil
}

func (c *fakeCam) GetPresets() ([]model.CameraPreset, error) {
	return c.presets, c.presetsErr
}

// fakeCamService hands out one camera and remembers what it was asked for.
type fakeCamService struct {
	cam        camera.Cam
	err        error
	gotAddress string
	gotType    model.CameraType
}

func (s *fakeCamService) For(address string, cameraType model.CameraType) (camera.Cam, error) {
	s.gotAddress = address
	s.gotType = cameraType

	if s.err != nil {
		return nil, s.err
	}

	return s.cam, nil
}

// The address and type come off the lecture hall, so a room's camera cannot be
// reached with another room's credentials.
func TestCameraForUsesTheLectureHallsCamera(t *testing.T) {
	cam := &fakeCam{}
	cams := &fakeCamService{cam: cam}
	a := New(nil, WithCamService(cams))

	got, err := a.cameraFor(model.LectureHall{CameraIP: "10.0.0.5", CameraType: model.Panasonic})
	if err != nil {
		t.Fatalf("cameraFor: %v", err)
	}

	if got != camera.Cam(cam) {
		t.Error("returned a camera other than the one the service handed out")
	}

	if cams.gotAddress != "10.0.0.5" {
		t.Errorf("asked for camera at %q, want 10.0.0.5", cams.gotAddress)
	}

	if cams.gotType != model.Panasonic {
		t.Errorf("asked for a %v camera, want %v", cams.gotType, model.Panasonic)
	}
}

// Each failure is a different kind of problem, and the status is how a caller tells
// "this room has no camera" from "the camera is not answering".
func TestCameraForFailures(t *testing.T) {
	unreachable := errors.New("dial tcp: connection refused")

	tests := []struct {
		name        string
		api         *API
		lectureHall model.LectureHall
		wantCode    codes.Code
		wantMessage string
	}{
		{
			name:        "no camera service",
			api:         New(nil),
			lectureHall: model.LectureHall{CameraIP: "10.0.0.5"},
			wantCode:    codes.Unknown,
			wantMessage: errNoCamService.Error(),
		},
		{
			name:        "lecture hall without a camera",
			api:         New(nil, WithCamService(&fakeCamService{cam: &fakeCam{}})),
			lectureHall: model.LectureHall{Name: "Audimax"},
			wantCode:    codes.InvalidArgument,
			wantMessage: errNoCameraInLectureHall.Error(),
		},
		{
			name:        "camera not answering",
			api:         New(nil, WithCamService(&fakeCamService{err: unreachable})),
			lectureHall: model.LectureHall{CameraIP: "10.0.0.5"},
			wantCode:    codes.Unavailable,
			wantMessage: unreachable.Error(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cam, err := tt.api.cameraFor(tt.lectureHall)
			if err == nil {
				t.Fatal("got a camera, want an error")
			}

			if cam != nil {
				t.Error("returned a camera alongside the error")
			}

			// e.WithStatus rebuilds the error as a status error rather than
			// wrapping it, so the message is what survives, not the identity.
			if got := status.Code(err); got != tt.wantCode {
				t.Errorf("code = %v, want %v", got, tt.wantCode)
			}

			if got := status.Convert(err).Message(); got != tt.wantMessage {
				t.Errorf("message = %q, want %q", got, tt.wantMessage)
			}
		})
	}
}

// A snapshot with nowhere to go would otherwise land in the working directory, where
// nothing serves it and nobody looks for it.
func TestSnapshotDir(t *testing.T) {
	configured := New(nil, WithPresetImageDir("/srv/static"))

	dir, err := configured.snapshotDir()
	if err != nil {
		t.Fatalf("snapshotDir: %v", err)
	}

	if dir != "/srv/static" {
		t.Errorf("snapshotDir is %q, want /srv/static", dir)
	}

	_, err = New(nil).snapshotDir()
	if got := status.Convert(err).Message(); got != errNoPresetImageDir.Error() {
		t.Errorf("unconfigured snapshotDir said %q, want %q", got, errNoPresetImageDir.Error())
	}
}
