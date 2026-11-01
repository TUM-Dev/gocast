package actions

import (
	"context"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/tum-dev/gocast/runner/config"
	"github.com/tum-dev/gocast/runner/pkg/ffmpeg"
	"github.com/tum-dev/gocast/runner/protobuf"
)

func TestKeepRanges(t *testing.T) {
	tests := []struct {
		name     string
		discard  []TimeRange
		duration float64
		want     []TimeRange
	}{
		{"nothing discarded", nil, 100, []TimeRange{{0, 100}}},
		{"middle", []TimeRange{{10, 20}}, 100, []TimeRange{{0, 10}, {20, 100}}},
		{"from the start", []TimeRange{{0, 20}}, 100, []TimeRange{{20, 100}}},
		{"to the end", []TimeRange{{80, 100}}, 100, []TimeRange{{0, 80}}},
		{"beyond the end is ignored", []TimeRange{{80, 100}, {110, 120}}, 100, []TimeRange{{0, 80}}},
		{"starts beyond the end", []TimeRange{{120, 130}}, 100, []TimeRange{{0, 100}}},
		{"unsorted", []TimeRange{{50, 60}, {10, 20}}, 100, []TimeRange{{0, 10}, {20, 50}, {60, 100}}},
		{"overlapping", []TimeRange{{10, 30}, {20, 40}}, 100, []TimeRange{{0, 10}, {40, 100}}},
		{"touching", []TimeRange{{10, 20}, {20, 30}}, 100, []TimeRange{{0, 10}, {30, 100}}},
		{"sliver between cuts is dropped", []TimeRange{{10, 20}, {20.05, 30}}, 100, []TimeRange{{0, 10}, {30, 100}}},
		{"everything", []TimeRange{{0, 100}}, 100, nil},
		{"more than everything", []TimeRange{{-5, 200}}, 100, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := keepRanges(tt.discard, tt.duration); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestKeepRangesDoesNotReorderInput(t *testing.T) {
	in := []TimeRange{{50, 60}, {10, 20}}
	keepRanges(in, 100)
	if in[0].Start != 50 {
		t.Errorf("input was sorted in place: %v", in)
	}
}

func TestCutFilter(t *testing.T) {
	keep := []TimeRange{{0, 10}, {20.5, 30}}
	t.Run("audio", func(t *testing.T) {
		got := cutFilter(keep, true)
		for _, want := range []string{
			"[0:v]trim=start=0.000:end=10.000,setpts=PTS-STARTPTS[v0];",
			"[0:a]atrim=start=20.500:end=30.000,asetpts=PTS-STARTPTS[a1];",
			"[v0][a0][v1][a1]concat=n=2:v=1:a=1[vc][a];",
			"[vc]scale=trunc(iw/2)*2:trunc(ih/2)*2[v]",
		} {
			if !strings.Contains(got, want) {
				t.Errorf("missing %q in\n%s", want, got)
			}
		}
	})
	t.Run("without audio", func(t *testing.T) {
		got := cutFilter(keep, false)
		if strings.Contains(got, "atrim") || !strings.Contains(got, "[v0][v1]concat=n=2:v=1:a=0[vc];") {
			t.Errorf("unexpected audio in\n%s", got)
		}
	})
}

func TestCutArgsKeepInputOneArgument(t *testing.T) {
	in := "/vod/a file; rm -rf -i x/playlist.m3u8"
	args := cutArgs(in, "/vod/1/X", []TimeRange{{0, 5}}, true)
	for i, a := range args {
		if a == "-i" && args[i+1] != in {
			t.Errorf("input arg = %q, want %q", args[i+1], in)
		}
	}
}

func TestCutVODMissingContext(t *testing.T) {
	notify := make(chan *protobuf.Notification, 1)
	full := map[string]any{"streamID": uint64(1), "sourceURL": "http://edge/vod/1/playlist.m3u8", "streamVersion": "STREAM_VERSION_COMBINED"}
	for _, missing := range []string{"streamID", "sourceURL", "streamVersion"} {
		d := map[string]any{}
		for k, v := range full {
			if k != missing {
				d[k] = v
			}
		}
		if err := CutVOD(context.Background(), testLogger(), notify, d, testBroker()); !IsAbortingError(err) {
			t.Errorf("without %s: got %v, want an aborting error", missing, err)
		}
	}
}

// importFixture lays down a 3 second VOD of stream 7 the way an upload would, and returns the
// storage it is in.
func importFixture(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not available")
	}
	storage := t.TempDir()
	config.Config.StoragePath = storage
	config.Config.EdgeServer = "http://edge"
	upload := makeUpload(t, "up.mp4", "-c:v", "libx264", "-pix_fmt", "yuv420p", "-c:a", "aac")
	d := map[string]any{"streamID": uint64(7), "streamVersion": "STREAM_VERSION_COMBINED", "uploadPath": upload}
	if err := ImportVOD(context.Background(), testLogger(), make(chan *protobuf.Notification, 64), d, testBroker()); err != nil {
		t.Fatalf("ImportVOD = %v", err)
	}
	return storage
}

func duration(t *testing.T, playlist string) float64 {
	t.Helper()
	probe, err := ffmpeg.Probe(context.Background(), playlist)
	if err != nil {
		t.Fatalf("probe %s: %v", playlist, err)
	}
	return probe.Duration()
}

func TestCutVOD(t *testing.T) {
	for _, tt := range []struct {
		name   string
		target uint64
	}{
		{"overwrite", 7},
		{"duplicate", 8},
	} {
		t.Run(tt.name, func(t *testing.T) {
			storage := importFixture(t)
			source := filepath.Join(storage, "7", "STREAM_VERSION_COMBINED", "playlist.m3u8")
			notify := make(chan *protobuf.Notification, 64)
			d := map[string]any{
				"streamID":      tt.target,
				"sourceURL":     source,
				"streamVersion": "STREAM_VERSION_COMBINED",
				"discard":       []TimeRange{{0.5, 1.5}, {2.5, 10}},
			}
			if err := CutVOD(context.Background(), testLogger(), notify, d, testBroker()); err != nil {
				t.Fatalf("CutVOD = %v", err)
			}

			// 3s minus the 1s and the 0.5s that are in the video.
			result := filepath.Join(storage, "8", "STREAM_VERSION_COMBINED", "playlist.m3u8")
			if tt.target == 7 {
				result = source
			}
			if got := duration(t, result); math.Abs(got-1.5) > 0.5 {
				t.Errorf("result is %.2fs long, want about 1.5s", got)
			}
			if tt.target == 8 {
				if got := duration(t, source); math.Abs(got-3) > 0.5 {
					t.Errorf("the source was changed, it is %.2fs long now", got)
				}
			}
			if got, _ := d["playlistURL"].(string); got != result {
				t.Errorf("playlistURL = %q, want %q", got, result)
			}

			var ready *protobuf.VODReadyNotification
			for len(notify) > 0 {
				if n := (<-notify).GetVodReady(); n != nil {
					ready = n
				}
			}
			if ready == nil || ready.GetStream().GetId() != tt.target || !strings.HasSuffix(ready.GetUrl(), fmt.Sprintf("/vod/%d/STREAM_VERSION_COMBINED/playlist.m3u8", tt.target)) {
				t.Errorf("VODReady = %v, want one for stream %d", ready, tt.target)
			}
			entries, _ := os.ReadDir(filepath.Join(storage, "7"))
			for _, e := range entries {
				if strings.HasPrefix(e.Name(), ".") {
					t.Errorf("work directory %s left behind", e.Name())
				}
			}
		})
	}
}

func TestCutVODNothingLeftKeepsVOD(t *testing.T) {
	storage := importFixture(t)
	notify := make(chan *protobuf.Notification, 64)
	d := map[string]any{
		"streamID":      uint64(7),
		"sourceURL":     filepath.Join(storage, "7", "STREAM_VERSION_COMBINED", "playlist.m3u8"),
		"streamVersion": "STREAM_VERSION_COMBINED",
		"discard":       []TimeRange{{0, 100}},
	}
	if err := CutVOD(context.Background(), testLogger(), notify, d, testBroker()); !IsAbortingError(err) {
		t.Fatalf("CutVOD = %v, want an aborting error", err)
	}
	failed := (<-notify).GetVodFailed()
	if failed == nil || failed.GetReason() != reasonNothingLeft {
		t.Errorf("notification = %v, want VODFailed with %q", failed, reasonNothingLeft)
	}
	if got := duration(t, filepath.Join(storage, "7", "STREAM_VERSION_COMBINED", "playlist.m3u8")); math.Abs(got-3) > 0.5 {
		t.Errorf("the VOD was changed, it is %.2fs long now", got)
	}
}
