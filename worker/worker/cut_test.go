package worker

import (
	"fmt"
	"strings"
	"testing"
)

func TestGetFfmpegArgs(t *testing.T) {
	c := Cutter{
		Input:  "in.mp4",
		Output: "out.mp4",
	}
	c.AddSection(0, 123)
	c.AddSection(245, 555)

	fmt.Println(strings.Join(append([]string{"ffmpeg"}, c.getFfmpegArgs()...), " "))
}
