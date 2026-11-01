package runner

import (
	"context"
	"reflect"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/tum-dev/gocast/runner/pkg/actions"
	"github.com/tum-dev/gocast/runner/pkg/ptr"
	"github.com/tum-dev/gocast/runner/protobuf"
)

func TestRegenerateActions(t *testing.T) {
	for _, tc := range []struct {
		silence, sprite bool
		want            int
	}{
		{false, false, 2},
		{true, false, 3},
		{false, true, 3},
		{true, true, 4},
	} {
		if got := len(regenerateActions(tc.silence, tc.sprite)); got != tc.want {
			t.Errorf("silence=%v sprite=%v: %d actions, want %d", tc.silence, tc.sprite, got, tc.want)
		}
	}
}

// A thumbnail that can't be made aborts the job, and the recordings that are redone are the
// ones that may well have none: the duration must not depend on it.
func TestRegenerateActionsProbeDurationBeforeThumbnail(t *testing.T) {
	for _, silence := range []bool{false, true} {
		got := regenerateActions(silence, true)
		if reflect.ValueOf(got[0]).Pointer() != reflect.ValueOf(actions.MkDuration).Pointer() {
			t.Errorf("detectSilence=%v: the first action is not MkDuration", silence)
		}
	}
}

func TestCutActions(t *testing.T) {
	if len(cutActions) != 3 || reflect.ValueOf(cutActions[0]).Pointer() != reflect.ValueOf(actions.CutVOD).Pointer() {
		t.Errorf("want CutVOD first of 3 actions, got %d", len(cutActions))
	}
}

func TestCutVODRejectsInvalidRequests(t *testing.T) {
	version := protobuf.StreamVersion_STREAM_VERSION_COMBINED
	valid := func() *protobuf.CutVODRequest {
		return &protobuf.CutVODRequest{
			StreamId:       ptr.Take(uint64(1)),
			TargetStreamId: ptr.Take(uint64(1)),
			Version:        ptr.Take(version),
			Discards:       []*protobuf.CutVODRequest_Range{{Start: ptr.Take(1.0), End: ptr.Take(2.0)}},
			PlaylistUrl:    ptr.Take("http://edge/vod/1/playlist.m3u8"),
		}
	}
	for name, mutate := range map[string]func(*protobuf.CutVODRequest){
		"no stream":      func(r *protobuf.CutVODRequest) { r.StreamId = nil },
		"no target":      func(r *protobuf.CutVODRequest) { r.TargetStreamId = nil },
		"no version":     func(r *protobuf.CutVODRequest) { r.Version = nil },
		"nothing to cut": func(r *protobuf.CutVODRequest) { r.Discards = nil },
		"no playlist":    func(r *protobuf.CutVODRequest) { r.PlaylistUrl = nil },
		"empty range":    func(r *protobuf.CutVODRequest) { r.Discards[0].End = ptr.Take(1.0) },
		"negative start": func(r *protobuf.CutVODRequest) { r.Discards[0].Start = ptr.Take(-1.0) },
	} {
		req := valid()
		mutate(req)
		_, err := (&Runner{}).CutVOD(context.Background(), req)
		if status.Code(err) != codes.InvalidArgument {
			t.Errorf("%s: got %v, want InvalidArgument", name, err)
		}
	}
}

func TestRequestWaveformRejectsInvalidRequests(t *testing.T) {
	for name, req := range map[string]*protobuf.WaveformRequest{
		"no stream":   {Sources: []*protobuf.WaveformRequest_Source{{Version: ptr.Take(protobuf.StreamVersion_STREAM_VERSION_COMBINED), PlaylistUrl: ptr.Take("http://edge/a.m3u8")}}},
		"no sources":  {StreamId: ptr.Take(uint64(1))},
		"unspecified": {StreamId: ptr.Take(uint64(1)), Sources: []*protobuf.WaveformRequest_Source{{Version: ptr.Take(protobuf.StreamVersion_STREAM_VERSION_UNSPECIFIED), PlaylistUrl: ptr.Take("http://edge/a.m3u8")}}},
		"no url":      {StreamId: ptr.Take(uint64(1)), Sources: []*protobuf.WaveformRequest_Source{{Version: ptr.Take(protobuf.StreamVersion_STREAM_VERSION_COMBINED)}}},
	} {
		if _, err := (&Runner{}).RequestWaveform(context.Background(), req); status.Code(err) != codes.InvalidArgument {
			t.Errorf("%s: got %v, want InvalidArgument", name, err)
		}
	}
}
