package ServerInterface

import (
	"context"
	"github.com/tum-dev/gocast/runner/protobuf"
)

type ServerInf interface {
	RequestSelfStream(ctx context.Context, request *protobuf.SelfStreamRequest) *protobuf.SelfStreamResponse
	NotifyVoDUploadFinished(ctx context.Context, request *protobuf.VoDUploadFinished) *protobuf.Status
	NotifySilenceResults(ctx context.Context, request *protobuf.SilenceResults) *protobuf.Status
	NotifyStreamStarted(ctx context.Context, request *protobuf.StreamStarted) *protobuf.Status
	NotifyStreamEnded(ctx context.Context, request *protobuf.StreamEnded) *protobuf.Status
	NotifyThumbnailsFinished(ctx context.Context, request *protobuf.ThumbnailsFinished) *protobuf.Status
	NotifyTranscodingFailure(ctx context.Context, request *protobuf.TranscodingFailureNotification) *protobuf.Status
	GetStreamInfoForUpload(ctx context.Context, request *protobuf.StreamInfoForUploadRequest) *protobuf.StreamInfoForUploadResponse
}
