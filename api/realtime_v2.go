package api

import (
	"context"

	protobuf "github.com/TUM-Dev/gocast/apiv2/protobuf/server"
	"github.com/TUM-Dev/gocast/pkg/realtimehub"
)

// TEMPORARY, deleted with api/: the v1 producers below the chat (live state, title,
// description) also publish to the v2 hub, so the SPA's socket hears about changes
// made through v1. The reverse direction is deliberately not built; the old UI's
// sockets are frozen and never see v2 events.
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
