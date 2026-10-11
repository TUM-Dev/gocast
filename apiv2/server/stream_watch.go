package apiv2

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	protobuf "github.com/TUM-Dev/gocast/apiv2/protobuf/server"
)

// The watch page's API is declared before it is implemented, so that the page can be
// built against the generated clients while the handlers are written. Each handler
// below answers Unimplemented until then; the access policies in services.go are
// already the final ones.

func errWatchNotImplemented(rpc string) error {
	return status.Error(codes.Unimplemented, rpc+" is declared but not implemented yet")
}

// GetStreamWatch returns everything the watch page renders in one call.
func (a *API) GetStreamWatch(context.Context, *protobuf.GetStreamWatchRequest) (*protobuf.StreamWatch, error) {
	return nil, errWatchNotImplemented("getStreamWatch")
}

// ListAllowedStreamReactions returns the emoji a viewer may react with.
func (a *API) ListAllowedStreamReactions(context.Context, *protobuf.ListAllowedStreamReactionsRequest) (*protobuf.ListAllowedStreamReactionsResponse, error) {
	return nil, errWatchNotImplemented("listAllowedStreamReactions")
}

// AddStreamReaction records the caller's reaction and tells the course's administrators.
func (a *API) AddStreamReaction(context.Context, *protobuf.AddStreamReactionRequest) (*emptypb.Empty, error) {
	return nil, errWatchNotImplemented("addStreamReaction")
}

// ReportStreamIssue forwards a viewer's problem report to the alert room.
func (a *API) ReportStreamIssue(context.Context, *protobuf.ReportStreamIssueRequest) (*emptypb.Empty, error) {
	return nil, errWatchNotImplemented("reportStreamIssue")
}

// ReportStreamSeek counts a seek for the seek heatmap.
func (a *API) ReportStreamSeek(context.Context, *protobuf.ReportStreamSeekRequest) (*emptypb.Empty, error) {
	return nil, errWatchNotImplemented("reportStreamSeek")
}

// GetStreamSeekHeatmap returns where viewers jump to in a recording.
func (a *API) GetStreamSeekHeatmap(context.Context, *protobuf.GetStreamSeekHeatmapRequest) (*protobuf.GetStreamSeekHeatmapResponse, error) {
	return nil, errWatchNotImplemented("getStreamSeekHeatmap")
}

// SearchStreamSubtitles searches the subtitles of one recording.
func (a *API) SearchStreamSubtitles(context.Context, *protobuf.SearchStreamSubtitlesRequest) (*protobuf.SearchStreamSubtitlesResponse, error) {
	return nil, errWatchNotImplemented("searchStreamSubtitles")
}

// EndStream ends a live stream early, keeping or discarding what was recorded.
func (a *API) EndStream(context.Context, *protobuf.EndStreamRequest) (*emptypb.Empty, error) {
	return nil, errWatchNotImplemented("endStream")
}
