package tools

import (
	"github.com/livekit/protocol/auth"
	lksdk "github.com/livekit/server-sdk-go"
)

var LivekitRoomClient *lksdk.RoomServiceClient
var LivekitEgressClient *lksdk.EgressClient

func InitLivekitClients() {
	LivekitRoomClient = lksdk.NewRoomServiceClient(Cfg.Livekit.Host, Cfg.Livekit.ApiKey, Cfg.Livekit.Secret)
	LivekitEgressClient = lksdk.NewEgressClient(Cfg.Livekit.Host, Cfg.Livekit.ApiKey, Cfg.Livekit.Secret)
}

func GenerateLivekitAuthToken(identity string, room string) (string, error) {
	at := auth.NewAccessToken(Cfg.Livekit.ApiKey, Cfg.Livekit.Secret)
	grant := &auth.VideoGrant{
		RoomJoin:  true,
		Room:      room,
		RoomAdmin: true,
	}
	at.AddGrant(grant).SetIdentity(identity)
	return at.ToJWT()
}
