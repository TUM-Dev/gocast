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

// ViewerCounter is what live statistics need from the v2 hub; realtimehub.Hub
// satisfies it.
type ViewerCounter interface {
	Viewers(streamID uint) int
}

// viewerCount is the stream's total audience: v1 sockets plus v2 sockets. Adding is
// not double counting because a client holds one kind only: the old templates open
// just the v1 socket and the SPA just the v2 one, and each page is served by exactly
// one of them. v2 may be nil (tests, or before startup wiring).
func viewerCount(streamID uint, v2 ViewerCounter) int {
	n := len(sessionsFor(streamID))
	if v2 != nil {
		n += v2.Viewers(streamID)
	}
	return n
}

// V1Viewers is the number of v1 sockets on the stream. The v2 hub adds it to the
// count in its ViewersEvent so the SPA shows the true total.
func V1Viewers(streamID uint) int {
	return len(sessionsFor(streamID))
}
