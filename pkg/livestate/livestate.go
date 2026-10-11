// Package livestate is the glue between whatever knows a stream's state (the runner
// manager, the stats cron) and the realtime hub the SPA's socket listens on.
package livestate

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/model"
	"github.com/TUM-Dev/gocast/pkg/realtimehub"
)

// hub is the realtime hub live-state changes are published to. It stays nil until
// SetRealtimeHub runs, which keeps tests that never start a hub working.
var hub realtimehub.Hub

// SetRealtimeHub is called once at startup, before anything is served.
func SetRealtimeHub(h realtimehub.Hub) {
	hub = h
}

var logger = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
	Level: slog.LevelDebug,
})).With("service", "livestate")

// ViewerCounter is what live statistics need from the v2 hub; realtimehub.Hub
// satisfies it.
type ViewerCounter interface {
	Viewers(streamID uint) int
}

// NotifyViewersLiveState tells the stream's viewers it went live or stopped.
func NotifyViewersLiveState(streamID uint, live bool) {
	if hub != nil {
		hub.Publish(context.Background(), streamID, realtimehub.LiveEvent(streamID, live))
	}
}

// CollectStats records the viewer count of every live stream once per tick. Only live
// streams get a row; the count is the v2 hub's.
func CollectStats(daoWrapper dao.DaoWrapper, viewers ViewerCounter) func() {
	return func() {
		live, err := daoWrapper.StreamsDao.GetCurrentLive(context.Background())
		if err != nil {
			logger.Error("Loading live streams for stats failed", "err", err)
			return
		}
		for _, s := range live {
			viewers := viewers.Viewers(s.ID)
			if viewers == 0 {
				continue
			}
			stat := model.Stat{
				Time:     time.Now(),
				StreamID: s.ID,
				Viewers:  uint(viewers),
				Live:     true,
			}
			if err := daoWrapper.AddStat(stat); err != nil {
				logger.Error("Saving stat failed", "err", err)
			}
		}
	}
}
