package tools

import (
	lksdk "github.com/livekit/server-sdk-go"
)

var LivekitRoomClient *lksdk.RoomServiceClient
var LivekitEgressClient *lksdk.EgressClient

func InitLivekitClients() {
	LivekitRoomClient = lksdk.NewRoomServiceClient(Cfg.Livekit.Host, Cfg.Livekit.ApiKey, Cfg.Livekit.Secret)
	LivekitEgressClient = lksdk.NewEgressClient(Cfg.Livekit.Host, Cfg.Livekit.ApiKey, Cfg.Livekit.Secret)
}
