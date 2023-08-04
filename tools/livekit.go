package tools

import (
	"context"
	"github.com/livekit/protocol/auth"
	"github.com/livekit/protocol/livekit"
	lksdk "github.com/livekit/server-sdk-go"
)

var LivekitRoomClient *lksdk.RoomServiceClient
var LivekitEgressClient *lksdk.EgressClient

type EgressInfo struct {
	Room     string
	EgressId string
}

var activeEgress []*EgressInfo

func InitLivekitClients() {
	LivekitRoomClient = lksdk.NewRoomServiceClient(Cfg.Livekit.Host, Cfg.Livekit.ApiKey, Cfg.Livekit.Secret)
	LivekitEgressClient = lksdk.NewEgressClient(Cfg.Livekit.Host, Cfg.Livekit.ApiKey, Cfg.Livekit.Secret)
	activeEgress = []*EgressInfo{}
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

func StartLivekitEgress(room string) (string, error) {
	for _, egress := range activeEgress {
		if egress.Room == room {
			return egress.EgressId, nil
		}
	}

	ctx := context.Background()

	fileRequest := &livekit.RoomCompositeEgressRequest{
		RoomName: room,
		Layout:   "speaker",
		Output: &livekit.RoomCompositeEgressRequest_Stream{
			Stream: &livekit.StreamOutput{
				Protocol: livekit.StreamProtocol_RTMP,
				Urls:     []string{"rtmp://host.docker.internal:1935/live/rfBd56ti2SMtYvSgD5xAV0YU99zampta7Z7S575KLkIZ9PYk"},
			},
		},
	}

	info, err := LivekitEgressClient.StartRoomCompositeEgress(ctx, fileRequest)
	if err != nil {
		return "", err
	}

	activeEgress = append(activeEgress, &EgressInfo{
		Room:     room,
		EgressId: info.EgressId,
	})
	return info.EgressId, err
}
