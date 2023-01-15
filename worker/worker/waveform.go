package worker

import (
	"fmt"
	uuid "github.com/iris-contrib/go.uuid"
	"github.com/joschahenningsen/TUM-Live/worker/pb"
	log "github.com/sirupsen/logrus"
	"io"
	"os"
	"os/exec"
)

const (
	waveFormWidth  = 2000
	waveFormHeight = 230
)

// GetWaveform returns the waveform of a given video as byte slice
func GetWaveform(request *pb.WaveformRequest) ([]byte, error) {
	log.Info("GetWaveform ", request.File)
	v4, err := uuid.NewV4()
	if err != nil {
		return nil, err
	}
	tempFile := "/tmp/" + v4.String() + ".png"
	c := []string{"ffmpeg", "-i", request.File,
		"-filter_complex", fmt.Sprintf("aformat=channel_layouts=mono,showwavespic=s=%dx%d:colors=white|white:scale=lin", waveFormWidth, waveFormHeight),
		"-frames:v", "1",
		tempFile,
	}
	cmd := exec.Command("nice", c...)
	log.Info(cmd.String())
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.WithField("combinedOutput", string(output)).Error("Could not get waveform with ffmpeg")
		return nil, err
	}
	f, err := os.Open(tempFile)
	if err != nil {
		return nil, err
	}
	bytes, err := io.ReadAll(f)
	if err != nil {
		return nil, err
	}
	f.Close()
	err = os.Remove(tempFile)
	if err != nil {
		log.WithError(err).Error("Could not remove temp waveform file")
	}
	log.Info("GetWaveform done")
	return bytes, nil
}
