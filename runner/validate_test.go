package runner

import (
	"testing"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/tum-dev/gocast/runner/pkg/ptr"
	"github.com/tum-dev/gocast/runner/protobuf"
)

func validRequest() *protobuf.StreamRequest {
	return &protobuf.StreamRequest{
		StreamId:            ptr.Take(uint64(1)),
		Version:             ptr.Take(protobuf.StreamVersion_STREAM_VERSION_COMBINED),
		End:                 timestamppb.New(time.Now().Add(time.Hour)),
		FfmpegOutputOptions: ptr.Take("-c:v libx264 -preset veryfast -tune zerolatency -c:a aac -ar 44100 -b:a 128k -g 60 -bufsize 5000k -maxrate 5000k -b:v 3500k"),
		Input:               ptr.Take("rtsp://user:pw@cam.example.org:554/stream"),
	}
}

// Everything gocast's requestStreamVersion actually sends must pass.
func TestValidateStreamRequestAcceptsGocastRequests(t *testing.T) {
	cases := map[string]func(*protobuf.StreamRequest){
		"lecture hall rtsp": func(*protobuf.StreamRequest) {},
		"lecture hall srt": func(r *protobuf.StreamRequest) {
			r.Input = ptr.Take("srt://10.0.0.5:9000")
			r.FfmpegOutputOptions = ptr.Take("-c:a copy -c:v copy -preset veryfast -tune zerolatency")
		},
		"rtsp copy": func(r *protobuf.StreamRequest) {
			r.FfmpegOutputOptions = ptr.Take("-c:a copy -c:v copy -rtsp_transport tcp -preset veryfast -tune zerolatency")
			r.FfmpegGlobalOptions = ptr.Take("-rtsp_transport tcp")
		},
		"self stream": func(r *protobuf.StreamRequest) {
			r.Input = ptr.Take("rtmp://ingest.tum.live/eidi-1234")
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			r := validRequest()
			mutate(r)
			if err := validateStreamRequest(r); err != nil {
				t.Fatalf("want accepted, got %v", err)
			}
		})
	}
}

func TestValidateStreamRequestRejects(t *testing.T) {
	cases := map[string]func(*protobuf.StreamRequest){
		"file input":   func(r *protobuf.StreamRequest) { r.Input = ptr.Take("/etc/passwd") },
		"file scheme":  func(r *protobuf.StreamRequest) { r.Input = ptr.Take("file:///etc/passwd") },
		"http input":   func(r *protobuf.StreamRequest) { r.Input = ptr.Take("http://169.254.169.254/latest") },
		"data input":   func(r *protobuf.StreamRequest) { r.Input = ptr.Take("data:text/plain,hi") },
		"empty input":  func(r *protobuf.StreamRequest) { r.Input = nil },
		"second input": func(r *protobuf.StreamRequest) { r.FfmpegInputOptions = ptr.Take("-f data -i /etc/shadow") },
		"extra output path": func(r *protobuf.StreamRequest) {
			r.FfmpegOutputOptions = ptr.Take("-map 0 -c copy -f data storage/live/1/x/leak")
		},
		"filter file": func(r *protobuf.StreamRequest) { r.FfmpegOutputOptions = ptr.Take("-lavfi movie=secret.mp4") },
		"global path": func(r *protobuf.StreamRequest) { r.FfmpegGlobalOptions = ptr.Take("-report -y ../x") },
		"no end":      func(r *protobuf.StreamRequest) { r.End = nil },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			r := validRequest()
			mutate(r)
			if err := validateStreamRequest(r); err == nil {
				t.Fatal("want rejection, got nil")
			}
		})
	}
}

func TestValidatePlaylistURL(t *testing.T) {
	for _, ok := range []string{"http://edge:8089/r1/1/STREAM_VERSION_COMBINED/playlist.m3u8", "https://live.rbg.tum.de/x.m3u8"} {
		if err := validatePlaylistURL(ok); err != nil {
			t.Errorf("%q: want accepted, got %v", ok, err)
		}
	}
	for _, bad := range []string{"", "/mass/1.m3u8", "file:///mass/1.m3u8", "rtsp://cam/x", "http:///nohost"} {
		if err := validatePlaylistURL(bad); err == nil {
			t.Errorf("%q: want rejection", bad)
		}
	}
}
