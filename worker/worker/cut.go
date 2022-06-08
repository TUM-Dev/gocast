package worker

import (
	"fmt"
	"strings"
)

type VideoSection struct {
	start int
	end   int
}

func (s VideoSection) GetStart() int {
	return s.start
}

func (s VideoSection) GetDuration() int {
	return s.end - s.start
}

type Cutter struct {
	Input    string
	Output   string
	sections []VideoSection
}

func (c *Cutter) AddSection(start int, end int) {
	c.sections = append(c.sections, VideoSection{
		start: start,
		end:   end,
	})
}

func (c *Cutter) getFfmpegArgs() []string {
	res := []string{"-y",
		"-i", c.Input,
		"-filter_complex",
	}

	vPads := make([]string, len(c.sections))
	aPads := make([]string, len(c.sections))
	for i := range vPads {
		vPads[i] = fmt.Sprintf("[v%d]", i)
		aPads[i] = fmt.Sprintf("[a%d]", i)
	}

	var filterClauses []string
	for i, section := range c.sections {
		// video
		filterClauses = append(filterClauses, fmt.Sprintf("[0:v]trim=%d.00:duration=%d.00,setpts=PTS-STARTPTS;[v%d]",
			section.GetStart(),
			section.GetDuration(),
			i,
		))
		// audio
		filterClauses = append(filterClauses, fmt.Sprintf("[0:a]atrim=%d.00:duration=%d.00,asetpts=PTS-STARTPTS;[a%d]",
			section.GetStart(),
			section.GetDuration(),
			i,
		))
	}
	res = append(res, strings.Join(filterClauses, ";"))

	// video and audio mappings
	res = append(res, strings.Join(vPads, "")+fmt.Sprintf("concat=n=%d:unsafe=1[ov0]", len(c.sections)))
	res = append(res, strings.Join(aPads, "")+fmt.Sprintf("concat=n=%d:v=0:a=1[oa0]", len(c.sections)))

	res = append(res, "-strict", "-2", "-preset", "faster", "-crf", "18", "-map", "[oa0]", "-map", "[ov0]", c.Output)
	return res
	// target: ffmpeg -y -i testvideo_320x180.mp4 -filter_complex [0:v]trim=0.00:duration=10.00,setpts=PTS-STARTPTS;[0:a]atrim=0.00:duration=10.00,asetpts=PTS-STARTPTS;[0:v]trim=25.00:duration=19.00,setpts=PTS-STARTPTS;[0:a]atrim=25.00:duration=19.00,asetpts=PTS-STARTPTS;[v0][v1]concat=n=2:unsafe=1[ov0];[a0][a1]concat=n=2:v=0:a=1[oa0] -strict -2 -preset faster -crf 18 -map [oa0] -map [ov0] mux.mp4
}
