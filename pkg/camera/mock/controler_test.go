package mock_camera_test

import (
	"errors"
	"testing"

	"go.uber.org/mock/gomock"

	"github.com/TUM-Dev/gocast/model"
	"github.com/TUM-Dev/gocast/pkg/camera"
	mock_camera "github.com/TUM-Dev/gocast/pkg/camera/mock"
)

// The generated mock stands in for a real camera in every test that touches lecture hall
// control. If it drifts out of sync with the interface -- a method added to Cam and not
// regenerated -- this stops compiling, which is the cheap durable form of that check.
var _ camera.Cam = (*mock_camera.MockCam)(nil)

func TestMockCamRecordsCalls(t *testing.T) {
	t.Run("it forwards arguments and returns the configured values", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		cam := mock_camera.NewMockCam(ctrl)

		wantPresets := []model.CameraPreset{{Name: "Tafel", PresetID: 1, IsDefault: true}}
		cam.EXPECT().SetPreset(7).Return(nil)
		cam.EXPECT().TakeSnapshot("/tmp/out").Return("snap.jpg", nil)
		cam.EXPECT().GetPresets().Return(wantPresets, nil)

		if err := cam.SetPreset(7); err != nil {
			t.Errorf("SetPreset returned %v, want nil", err)
		}
		filename, err := cam.TakeSnapshot("/tmp/out")
		if err != nil || filename != "snap.jpg" {
			t.Errorf("TakeSnapshot = (%q, %v), want (snap.jpg, nil)", filename, err)
		}
		presets, err := cam.GetPresets()
		if err != nil {
			t.Errorf("GetPresets returned %v, want nil", err)
		}
		if len(presets) != 1 || presets[0].Name != "Tafel" || presets[0].PresetID != 1 {
			t.Errorf("GetPresets = %v, want %v", presets, wantPresets)
		}
	})

	// Error returns must survive the mock's type assertions; a mock that swallowed them
	// would make every error-path test using it pass vacuously.
	t.Run("it propagates errors from every method", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		cam := mock_camera.NewMockCam(ctrl)
		boom := errors.New("camera offline")

		cam.EXPECT().SetPreset(gomock.Any()).Return(boom)
		cam.EXPECT().TakeSnapshot(gomock.Any()).Return("", boom)
		cam.EXPECT().GetPresets().Return(nil, boom)

		if err := cam.SetPreset(1); !errors.Is(err, boom) {
			t.Errorf("SetPreset error = %v, want %v", err, boom)
		}
		if filename, err := cam.TakeSnapshot("/tmp"); !errors.Is(err, boom) || filename != "" {
			t.Errorf("TakeSnapshot = (%q, %v), want (\"\", %v)", filename, err, boom)
		}
		if presets, err := cam.GetPresets(); !errors.Is(err, boom) || presets != nil {
			t.Errorf("GetPresets = (%v, %v), want (nil, %v)", presets, err, boom)
		}
	})

	// A mock that accepted any argument would hide a driver called with the wrong preset,
	// so the argument matcher is verified to actually discriminate.
	t.Run("it distinguishes arguments", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		cam := mock_camera.NewMockCam(ctrl)
		cam.EXPECT().SetPreset(1).Return(nil)
		cam.EXPECT().SetPreset(2).Return(errors.New("second"))

		if err := cam.SetPreset(1); err != nil {
			t.Errorf("SetPreset(1) = %v, want nil", err)
		}
		if err := cam.SetPreset(2); err == nil {
			t.Error("SetPreset(2) matched the expectation for 1")
		}
	})

	// Call counts are the other half of "records calls": an unmet expectation must fail.
	t.Run("it reports a call that never happened", func(t *testing.T) {
		reporter := &recordingReporter{}
		ctrl := gomock.NewController(reporter)
		cam := mock_camera.NewMockCam(ctrl)
		cam.EXPECT().SetPreset(1).Return(nil)
		ctrl.Finish()

		if !reporter.failed {
			t.Error("an unfulfilled expectation was not reported")
		}
	})
}

// recordingReporter captures gomock failures instead of failing the test, so the mock's
// own reporting can be asserted.
type recordingReporter struct {
	failed bool
}

func (r *recordingReporter) Errorf(format string, args ...any) { r.failed = true }
func (r *recordingReporter) Fatalf(format string, args ...any) { r.failed = true }
