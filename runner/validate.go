package runner

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/tum-dev/gocast/runner/protobuf"
)

// Only live sources. "file:" would let a caller read any file the runner can, and an http(s)
// input is an SSRF into whatever network the runner sits in.
var allowedInputSchemes = map[string]bool{
	"rtsp": true, "rtsps": true, "srt": true, "rtmp": true, "rtmps": true,
}

// ffmpeg options that would turn a stream job into a file read or write, or run a second
// input next to the one gocast asked for. The manager is authenticated, so this is a second
// line of defence against a compromised gocast or a token leak, not the only one.
var forbiddenOptions = map[string]bool{
	"-i": true, "-lavfi": true, "-filter_complex": true, "-filter_complex_script": true,
	"-filter_script": true, "-passlogfile": true, "-report": true, "-dump_attachment": true,
}

// validateStreamRequest checks that a StreamRequest can only ever produce the HLS output
// the runner builds itself: the input is a live source and no option names a path or a
// second input. gocast's requestStreamVersion produces nothing else, so a request that
// fails here didn't come from a healthy gocast.
func validateStreamRequest(req *protobuf.StreamRequest) error {
	if err := validateLiveInput(req.GetInput()); err != nil {
		return err
	}
	for name, opts := range map[string]string{
		"ffmpeg_global_options": req.GetFfmpegGlobalOptions(),
		"ffmpeg_input_options":  req.GetFfmpegInputOptions(),
		"ffmpeg_output_options": req.GetFfmpegOutputOptions(),
	} {
		if err := validateFfmpegOptions(opts); err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
	}
	if !req.GetEnd().IsValid() {
		return fmt.Errorf("end is not a valid timestamp")
	}
	return nil
}

func validateLiveInput(input string) error {
	u, err := url.Parse(input)
	if err != nil {
		return fmt.Errorf("input is not a url: %w", err)
	}
	if !allowedInputSchemes[strings.ToLower(u.Scheme)] || u.Host == "" {
		return fmt.Errorf("input must be a rtsp, srt or rtmp url with a host")
	}
	return nil
}

func validateFfmpegOptions(opts string) error {
	for _, tok := range strings.Fields(opts) {
		if forbiddenOptions[strings.ToLower(tok)] {
			return fmt.Errorf("option %q is not allowed", tok)
		}
		// Paths only appear in the output the runner builds itself. A slash in a caller
		// option is either an extra output file or a file-backed filter source.
		if strings.ContainsAny(tok, `/\`) {
			return fmt.Errorf("option %q must not contain a path", tok)
		}
	}
	return nil
}

// validatePlaylistURL only lets section images be cut from the HLS playlists gocast serves.
func validatePlaylistURL(playlist string) error {
	u, err := url.Parse(playlist)
	if err != nil {
		return fmt.Errorf("playlist url is not a url: %w", err)
	}
	if (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return fmt.Errorf("playlist url must be http(s) with a host")
	}
	return nil
}
