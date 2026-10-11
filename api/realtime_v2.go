package api

import (
	"context"
	"time"

	protobuf "github.com/TUM-Dev/gocast/apiv2/protobuf/server"
	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/model"
	"github.com/TUM-Dev/gocast/pkg/realtimehub"
)

// The v1 producers of live state, title and description publish to the v2 hub, so the
// SPA's socket hears about changes made through v1. Deleted with api/.
var realtimeHub realtimehub.Hub

// SetRealtimeHub is called once at startup, before anything is served.
func SetRealtimeHub(hub realtimehub.Hub) {
	realtimeHub = hub
}

// publishV2 is a no-op until SetRealtimeHub runs, which keeps v1's tests hub-free.
func publishV2(streamID uint, event *protobuf.RealtimeEvent) {
	if realtimeHub != nil {
		realtimeHub.Publish(context.Background(), streamID, event)
	}
}

// ViewerCounter is what live statistics need from the v2 hub; realtimehub.Hub
// satisfies it.
type ViewerCounter interface {
	Viewers(streamID uint) int
}

// NotifyViewersLiveState tells the stream's viewers it went live or stopped.
func NotifyViewersLiveState(streamID uint, live bool) {
	publishV2(streamID, realtimehub.LiveEvent(streamID, live))
}

// CollectStats records the viewer count of every live stream once per tick. Only live
// streams get a row; the count is the v2 hub's.
func CollectStats(daoWrapper dao.DaoWrapper, v2 ViewerCounter) func() {
	return func() {
		live, err := daoWrapper.StreamsDao.GetCurrentLive(context.Background())
		if err != nil {
			logger.Error("Loading live streams for stats failed", "err", err)
			return
		}
		for _, s := range live {
			viewers := v2.Viewers(s.ID)
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
