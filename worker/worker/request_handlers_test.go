package worker

import (
	"github.com/joschahenningsen/TUM-Live/worker/pb"
	"os/exec"
	"syscall"
	"testing"
	"time"

	"github.com/joschahenningsen/TUM-Live/worker/cfg"
)

var s StreamContext

func setup() {
	cfg.WorkerID = "123"
	s = StreamContext{
		courseSlug:          "eidi",
		teachingTerm:        "W",
		teachingYear:        2021,
		startTime:           time.Date(2021, 9, 23, 8, 0, 0, 0, time.Local),
		streamId:            1,
		streamVersion:       "COMB",
		publishVoD:          true,
		stream:              true,
		endTime:             time.Now().Add(time.Hour),
		commands:            nil,
		thumbnailSpritePath: "/tmp/thumbnail_sprite.png",
	}
	cfg.TempDir = "/recordings"
}

func TestGetTranscodingFileName(t *testing.T) {
	setup()
	transcodingNameShould := "/mass/2021/W/eidi/2021-09-23_08-00/eidi-2021-09-23-08-00COMB.mp4"
	if got := s.getTranscodingFileName(); got != transcodingNameShould {
		t.Errorf("Wrong transcoding name, should be %s but is %s", transcodingNameShould, got)
	}
}

func TestGetRecordingFileName(t *testing.T) {
	setup()
	recordingNameShould := "/recordings/eidi-2021-09-23-08-00COMB.ts"
	if got := s.getRecordingFileName(); got != recordingNameShould {
		t.Errorf("Wrong recording name, should be %s but is %s", recordingNameShould, got)
	}
}

func TestThumbnailCreation(t *testing.T) {
	setup()
	err := createThumbnailSprite(&s)
	if err != nil {
		return
	}
}

// TestStreamEndRequest tests whether the process of a streamContext gets terminated when ending a stream via request
func TestStreamEndRequest(t *testing.T) {
	timeout := time.After(2 * time.Second)
	done := make(chan bool)
	go func() {
		const maxIterations int = 16

		request := pb.EndStreamRequest{
			StreamID:   s.streamId,
			WorkerID:   cfg.WorkerID,
			DiscardVoD: true,
		}
		for i := 0; i < maxIterations; i++ {
			request.StreamID = uint32(i % 4)
			s.streamId = request.StreamID
			// We have to create a new process each iteration, cat blocks without any arguments
			s.streamCmd = exec.Command("cat")
			s.streamCmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true} // See stream.go
			regularStreams.addContext(s.streamId, &s)
			err := s.streamCmd.Start()
			if err != nil {
				t.Errorf("Starting the streamCmd failed")
				return
			}
			// Should end the streamCmd process
			HandleStreamEndRequest(&request)
			wait, err := s.streamCmd.Process.Wait()
			if err != nil {
				t.Errorf("Wait did not succeed")
				return
			}
			// Process returns -1 when terminated via signal
			if wait.ExitCode() != -1 {
				t.Errorf("Process did not exit properly")
				return
			}
			if len(regularStreams.streams) > 0 {
				t.Errorf("Streams were not deleted properly")
				return
			}
		}
		done <- true
	}()

	select {
	case <-timeout:
		t.Fatal("Test didn't finish in time")
	case <-done:
	}
}
