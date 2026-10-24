package apiv2

import (
	"reflect"
	"testing"

	"github.com/TUM-Dev/gocast/model"
)

func TestPostProcessingVersions(t *testing.T) {
	for name, tc := range map[string]struct {
		stream model.Stream
		want   []postProcessingVersion
	}{
		"no recording": {model.Stream{}, nil},
		"all versions": {
			model.Stream{PlaylistUrl: "c", PlaylistUrlCAM: "m", PlaylistUrlPRES: "p"},
			[]postProcessingVersion{{"c", model.COMB}, {"m", model.CAM}, {"p", model.PRES}},
		},
		"camera only": {
			model.Stream{PlaylistUrlCAM: "m"},
			[]postProcessingVersion{{"m", model.CAM}},
		},
		"camera and slides": {
			model.Stream{PlaylistUrlCAM: "m", PlaylistUrlPRES: "p"},
			[]postProcessingVersion{{"m", model.CAM}, {"p", model.PRES}},
		},
	} {
		t.Run(name, func(t *testing.T) {
			if got := postProcessingVersions(tc.stream); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}
