// Package probe provides tooling to probe videos using ffprobe
package probe

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
)

// Probe gets a P from input
func Probe(ctx context.Context, input string) (P, error) {
	var res P
	cmd := exec.CommandContext(ctx, "ffprobe", "-v", "quiet", "-print_format", "json", "-show_format", "-show_streams", input)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return res, err
	}
	err = json.Unmarshal(out, &res)
	if err != nil {
		return res, fmt.Errorf("unmarshal: %w", err)
	}
	return res, nil
}

// P is the Probe result
type P struct {
	Streams []Stream `json:"streams"`
	Format  struct {
		Filename       string `json:"filename"`
		NbStreams      int    `json:"nb_streams"`
		NbPrograms     int    `json:"nb_programs"`
		NbStreamGroups int    `json:"nb_stream_groups"`
		FormatName     string `json:"format_name"`
		FormatLongName string `json:"format_long_name"`
		StartTime      string `json:"start_time"`
		Duration       string `json:"duration"`
		Size           string `json:"size"`
		BitRate        string `json:"bit_rate"`
		ProbeScore     int    `json:"probe_score"`
		Tags           struct {
			Encoder string `json:"encoder"`
		} `json:"tags"`
	} `json:"format"`
}

type Stream struct {
	Index              int    `json:"index"`
	CodecName          string `json:"codec_name"`
	CodecLongName      string `json:"codec_long_name"`
	Profile            string `json:"profile,omitempty"`
	CodecType          string `json:"codec_type"`
	CodecTagString     string `json:"codec_tag_string"`
	CodecTag           string `json:"codec_tag"`
	Width              int    `json:"width,omitempty"`
	Height             int    `json:"height,omitempty"`
	CodedWidth         int    `json:"coded_width,omitempty"`
	CodedHeight        int    `json:"coded_height,omitempty"`
	ClosedCaptions     int    `json:"closed_captions,omitempty"`
	FilmGrain          int    `json:"film_grain,omitempty"`
	HasBFrames         int    `json:"has_b_frames,omitempty"`
	SampleAspectRatio  string `json:"sample_aspect_ratio,omitempty"`
	DisplayAspectRatio string `json:"display_aspect_ratio,omitempty"`
	PixFmt             string `json:"pix_fmt,omitempty"`
	Level              int    `json:"level,omitempty"`
	ColorRange         string `json:"color_range,omitempty"`
	ColorSpace         string `json:"color_space,omitempty"`
	ColorTransfer      string `json:"color_transfer,omitempty"`
	ColorPrimaries     string `json:"color_primaries,omitempty"`
	ChromaLocation     string `json:"chroma_location,omitempty"`
	FieldOrder         string `json:"field_order,omitempty"`
	Refs               int    `json:"refs,omitempty"`
	IsAvc              string `json:"is_avc,omitempty"`
	NalLengthSize      string `json:"nal_length_size,omitempty"`
	RFrameRate         string `json:"r_frame_rate"`
	AvgFrameRate       string `json:"avg_frame_rate"`
	TimeBase           string `json:"time_base"`
	StartPts           int    `json:"start_pts"`
	StartTime          string `json:"start_time"`
	BitsPerRawSample   string `json:"bits_per_raw_sample,omitempty"`
	ExtradataSize      int    `json:"extradata_size,omitempty"`
	Disposition        struct {
		Default         int `json:"default"`
		Dub             int `json:"dub"`
		Original        int `json:"original"`
		Comment         int `json:"comment"`
		Lyrics          int `json:"lyrics"`
		Karaoke         int `json:"karaoke"`
		Forced          int `json:"forced"`
		HearingImpaired int `json:"hearing_impaired"`
		VisualImpaired  int `json:"visual_impaired"`
		CleanEffects    int `json:"clean_effects"`
		AttachedPic     int `json:"attached_pic"`
		TimedThumbnails int `json:"timed_thumbnails"`
		NonDiegetic     int `json:"non_diegetic"`
		Captions        int `json:"captions"`
		Descriptions    int `json:"descriptions"`
		Metadata        int `json:"metadata"`
		Dependent       int `json:"dependent"`
		StillImage      int `json:"still_image"`
		Multilayer      int `json:"multilayer"`
	} `json:"disposition"`
	Tags struct {
		BPS                      string `json:"BPS"`
		DURATION                 string `json:"DURATION"`
		NUMBEROFFRAMES           string `json:"NUMBER_OF_FRAMES"`
		NUMBEROFBYTES            string `json:"NUMBER_OF_BYTES"`
		STATISTICSWRITINGAPP     string `json:"_STATISTICS_WRITING_APP"`
		STATISTICSWRITINGDATEUTC string `json:"_STATISTICS_WRITING_DATE_UTC"`
		STATISTICSTAGS           string `json:"_STATISTICS_TAGS"`
		Language                 string `json:"language,omitempty"`
		Title                    string `json:"title,omitempty"`
	} `json:"tags"`
	SampleFmt      string `json:"sample_fmt,omitempty"`
	SampleRate     string `json:"sample_rate,omitempty"`
	Channels       int    `json:"channels,omitempty"`
	ChannelLayout  string `json:"channel_layout,omitempty"`
	BitsPerSample  int    `json:"bits_per_sample,omitempty"`
	InitialPadding int    `json:"initial_padding,omitempty"`
	BitRate        string `json:"bit_rate,omitempty"`
	DurationTs     int    `json:"duration_ts,omitempty"`
	Duration       string `json:"duration,omitempty"`
}

// DurationFloat returns the duration of the given Stream in seconds.
func (s Stream) DurationFloat() (float64, error) {
	return strconv.ParseFloat(s.Duration, 64)
}
