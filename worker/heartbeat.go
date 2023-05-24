package worker

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/joschahenningsen/TUM-Live/worker/protobuf"
)

func (w *Worker) sendHeartbeat() error {
	err := w.p.Update()
	if err != nil {
		return fmt.Errorf("could not update performance data: %w", err)
	}
	client, err := w.dialIn()
	if err != nil {
		return fmt.Errorf("could not connect to manager: %w", err)
	}
	uptimeStr := ""
	uptime := time.Since(w.startupTime).Round(time.Minute)
	if uptime == 0 {
		uptimeStr = time.Since(w.startupTime).Round(time.Second).String()
	} else {
		uptimeStr = strings.ReplaceAll(time.Since(w.startupTime).Round(time.Minute).String(), "0s", "")

	}
	_, err = client.Heartbeat(context.Background(), &protobuf.HeartbeatRequest{
		ID:       uint64(w.id),
		Workload: uint32(len(w.jobs)),
		Version:  w.version,
		CPU:      w.p.GetCpuStr(),
		Memory:   w.p.GetMemStr(),
		Disk:     w.p.GetDiskStr(),
		Uptime:   uptimeStr,
	})
	return err
}
