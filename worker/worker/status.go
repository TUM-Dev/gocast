package worker

import (
	"context"
	"fmt"
	"github.com/joschahenningsen/TUM-Live/worker/cfg"
	"github.com/joschahenningsen/TUM-Live/worker/pb"
	"github.com/joschahenningsen/TUM-Live/worker/worker/vmstat"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"strings"
	"sync"
	"time"
)

var statusLock = sync.RWMutex{}
var S *Status
var VersionTag string

const (
	costStream              = 3
	costTranscoding         = 2
	costSilenceDetection    = 1
	costThumbnailGeneration = 1
	costStream            = 3
	costTranscoding       = 2
	costSilenceDetection  = 1
	costKeywordExtraction = 1
)

type Status struct {
	workload  uint
	Jobs      []string
	StartTime time.Time

	// VM Metrics are updated regularly
	Stat *vmstat.VmStat
}

func (s *Status) startSilenceDetection(streamCtx *StreamContext) {
	defer s.SendHeartbeat()
	statusLock.Lock()
	s.workload += costSilenceDetection
	s.Jobs = append(s.Jobs, fmt.Sprintf("detecting silence in %s", streamCtx.getStreamName()))
	statusLock.Unlock()
}

func (s *Status) startStream(streamCtx *StreamContext) {
	defer s.SendHeartbeat()
	statusLock.Lock()
	notifyStreamStart(streamCtx)
	defer statusLock.Unlock()
	s.workload += costStream
	s.Jobs = append(s.Jobs, fmt.Sprintf("streaming %s", streamCtx.getStreamName()))
}

func (s *Status) startRecording(name string) {
	defer s.SendHeartbeat()
	statusLock.Lock()
	defer statusLock.Unlock()
	s.workload += costStream
	s.Jobs = append(s.Jobs, fmt.Sprintf("recording %s", name))
}

func (s *Status) startTranscoding(name string) {
	defer s.SendHeartbeat()
	statusLock.Lock()
	defer statusLock.Unlock()
	s.workload += costTranscoding
	s.Jobs = append(s.Jobs, fmt.Sprintf("transcoding %s", name))
}

func (s *Status) startThumbnailGeneration(streamCtx *StreamContext) {
	defer s.SendHeartbeat()
	statusLock.Lock()
	defer statusLock.Unlock()
	s.workload += costThumbnailGeneration
	s.Jobs = append(s.Jobs, fmt.Sprintf("generating thumbnail for %s", streamCtx.getTranscodingFileName()))
}

func (s *Status) endStream(streamCtx *StreamContext) {
	defer s.SendHeartbeat()
	statusLock.Lock()
	s.workload -= costStream
	for i := range s.Jobs {
		if s.Jobs[i] == fmt.Sprintf("streaming %s", streamCtx.getStreamName()) {
			s.Jobs = append(s.Jobs[:i], s.Jobs[i+1:]...)
			break
		}
	}
	statusLock.Unlock()
}

func (s *Status) endRecording(name string) {
	defer s.SendHeartbeat()
	statusLock.Lock()
	s.workload -= costStream
	for i := range s.Jobs {
		if s.Jobs[i] == fmt.Sprintf("recording %s", name) {
			s.Jobs = append(s.Jobs[:i], s.Jobs[i+1:]...)
			break
		}
	}
	statusLock.Unlock()
}

func (s *Status) endTranscoding(name string) {
	defer s.SendHeartbeat()
	statusLock.Lock()
	s.workload -= costTranscoding
	for i := range s.Jobs {
		if s.Jobs[i] == fmt.Sprintf("transcoding %s", name) {
			s.Jobs = append(s.Jobs[:i], s.Jobs[i+1:]...)
			break
		}
	}
	statusLock.Unlock()
}

func (s *Status) endThumbnailGeneration(streamContext *StreamContext) {
	defer s.SendHeartbeat()
	statusLock.Lock()
	s.workload -= costThumbnailGeneration
	for i := range s.Jobs {
		if s.Jobs[i] == fmt.Sprintf("generating thumbnail for %s", streamContext.getTranscodingFileName()) {
			s.Jobs = append(s.Jobs[:i], s.Jobs[i+1:]...)
			break
		}
	}
	statusLock.Unlock()
}

func (s *Status) endSilenceDetection(streamCtx *StreamContext) {
	defer s.SendHeartbeat()
	statusLock.Lock()
	s.workload -= costSilenceDetection
	for i := range s.Jobs {
		if s.Jobs[i] == fmt.Sprintf("detecting silence in %s", streamCtx.getStreamName()) {
			s.Jobs = append(s.Jobs[:i], s.Jobs[i+1:]...)
			break
		}
	}
	statusLock.Unlock()
}

func (s *Status) SendHeartbeat() {
	// WithInsecure: workerId used for authentication, all servers are inside their own VLAN to further improve security
	clientConn, err := grpc.Dial(fmt.Sprintf("%s:50052", cfg.MainBase), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.WithError(err).Error("unable to dial for heartbeat")
		return
	}
	client := pb.NewFromWorkerClient(clientConn)
	defer func(clientConn *grpc.ClientConn) {
		err := clientConn.Close()
		if err != nil {
			log.WithError(err).Warn("Can't close status req")
		}
	}(clientConn)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	_, err = client.SendHeartBeat(ctx, &pb.HeartBeat{
		WorkerID: cfg.WorkerID,
		Workload: uint32(s.workload),
		Jobs:     s.Jobs,
		Version:  VersionTag,
		CPU:      s.Stat.GetCpuStr(),
		Memory:   s.Stat.GetMemStr(),
		Disk:     s.Stat.GetDiskStr(),
		Uptime:   strings.ReplaceAll(time.Since(s.StartTime).Round(time.Minute).String(), "0s", ""),
	})
	if err != nil {
		log.WithError(err).Error("Sending Heartbeat failed")
	}
}

func (s *Status) startKeywordExtraction(streamCtx *StreamContext) {
	defer s.SendHeartbeat()
	statusLock.Lock()
	defer statusLock.Unlock()
	s.workload += costKeywordExtraction
	s.Jobs = append(s.Jobs, fmt.Sprintf("extracting keywords for %s", streamCtx.getTranscodingFileName()))
}

func (s *Status) endKeywordExtraction(streamCtx *StreamContext) {
	defer s.SendHeartbeat()
	statusLock.Lock()
	s.workload -= costKeywordExtraction
	for i := range s.Jobs {
		if s.Jobs[i] == fmt.Sprintf("extracting keywords for %s", streamCtx.getTranscodingFileName()) {
			s.Jobs = append(s.Jobs[:i], s.Jobs[i+1:]...)
			break
		}
	}
	statusLock.Unlock()
}
