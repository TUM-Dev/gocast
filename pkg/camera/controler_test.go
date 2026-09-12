package camera

import (
	"testing"

	"github.com/TUM-Dev/gocast/model"
	"github.com/TUM-Dev/gocast/pkg/camera/axis"
	"github.com/TUM-Dev/gocast/pkg/camera/panasonic"
	"github.com/TUM-Dev/gocast/pkg/camera/sony"
)

// The camera type stored on a lecture hall decides which vendor protocol is spoken, so
// each mapping is pinned: picking the wrong driver leaves an unusable camera in a hall.
func TestServiceFor(t *testing.T) {
	auths := map[model.CameraType]string{
		model.Axis:         "axisuser:axispass",
		model.Panasonic:    "panauser:panapass",
		model.Sony_SRG_A40: "sonyuser:sonypass",
	}
	s := NewService(auths)

	tests := []struct {
		name       string
		cameraType model.CameraType
		assert     func(t *testing.T, c Cam)
	}{
		{
			name:       "an Axis lecture hall gets the Axis driver with its credentials",
			cameraType: model.Axis,
			assert: func(t *testing.T, c Cam) {
				cam, ok := c.(*axis.AxisCam)
				if !ok {
					t.Fatalf("got %T, want *axis.AxisCam", c)
				}
				if cam.Ip != "10.0.0.1" || cam.Auth != "axisuser:axispass" {
					t.Errorf("got (%q, %q), want (10.0.0.1, axisuser:axispass)", cam.Ip, cam.Auth)
				}
			},
		},
		{
			name:       "a Panasonic lecture hall gets the Panasonic driver",
			cameraType: model.Panasonic,
			assert: func(t *testing.T, c Cam) {
				cam, ok := c.(*panasonic.PanasonicCam)
				if !ok {
					t.Fatalf("got %T, want *panasonic.PanasonicCam", c)
				}
				if cam.Auth != "panauser:panapass" {
					t.Errorf("auth = %q, want panauser:panapass", cam.Auth)
				}
			},
		},
		{
			name:       "a Sony lecture hall gets the Sony driver",
			cameraType: model.Sony_SRG_A40,
			assert: func(t *testing.T, c Cam) {
				cam, ok := c.(*sony.SonySRG)
				if !ok {
					t.Fatalf("got %T, want *sony.SonySRG", c)
				}
				if cam.Auth != "sonyuser:sonypass" {
					t.Errorf("auth = %q, want sonyuser:sonypass", cam.Auth)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := s.For("10.0.0.1", tt.cameraType)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			tt.assert(t, c)
		})
	}

	// A lecture hall carrying a camera type this build does not know (a stale row, a
	// removed vendor) must error rather than hand back a nil Cam that callers dereference.
	t.Run("an unknown camera type errors and returns no driver", func(t *testing.T) {
		for _, ct := range []model.CameraType{0, 99} {
			c, err := s.For("10.0.0.1", ct)
			if err == nil {
				t.Errorf("type %d: expected an error", ct)
			}
			if c != nil {
				t.Errorf("type %d: got driver %v, want nil", ct, c)
			}
		}
	})

	// BUG: a camera type with no configured credentials yields an empty auth string,
	// which protocol.MakeAuthenticatedRequest then splits on ":" and indexes at 1 --
	// panicking on the first request. The factory itself still succeeds.
	t.Run("a camera type without credentials yields an empty auth rather than an error", func(t *testing.T) {
		c, err := NewService(map[model.CameraType]string{}).For("10.0.0.1", model.Axis)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		cam, ok := c.(*axis.AxisCam)
		if !ok {
			t.Fatalf("got %T, want *axis.AxisCam", c)
		}
		if cam.Auth != "" {
			t.Errorf("auth = %q, want empty (see BUG note)", cam.Auth)
		}
	})

	t.Run("a nil auth map is tolerated", func(t *testing.T) {
		if _, err := NewService(nil).For("10.0.0.1", model.Axis); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}
