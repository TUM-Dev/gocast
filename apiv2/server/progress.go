package apiv2

import (
	"context"
	"errors"
	"net/http"

	e "github.com/TUM-Dev/gocast/apiv2/errors"
	h "github.com/TUM-Dev/gocast/apiv2/helpers"
	protobuf "github.com/TUM-Dev/gocast/apiv2/protobuf/server"
	"github.com/TUM-Dev/gocast/model"
)

// GetProgressBatch returns a batch of watch progresses for a list of streams for the current user
func (a *API) GetProgressBatch(ctx context.Context, req *protobuf.GetProgressBatchRequest) (*protobuf.GetProgressBatchResponse, error) {
	user, err := a.getCurrent(ctx)
	if err != nil {
		return nil, err
	}

	ids := req.StreamIds
	if len(ids) == 0 {
		return nil, e.WithStatus(http.StatusBadRequest, errors.New("No stream IDs provided"))
	}

	// Filter in the database: a user can have thousands of progresses, of which a
	// request asks for a handful.
	streamIDs := make([]uint, len(ids))
	for i, id := range ids {
		streamIDs[i] = uint(id)
	}

	progressBatch, err := a.dao.LoadProgress(user.ID, streamIDs)
	if err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}

	progressResults := make([]*protobuf.StreamProgress, 0, len(progressBatch))
	for _, progress := range progressBatch {
		progressResults = append(progressResults, h.ParseStreamProgressToProto(progress))
	}

	return &protobuf.GetProgressBatchResponse{ProgressBatch: progressResults}, nil
}

// UpdateProgress updates the watch progress for a stream
func (a *API) UpdateProgress(ctx context.Context, req *protobuf.UpdateProgressRequest) (*protobuf.StreamProgress, error) {
	user, stream, _, err := a.authorizeUserForStreamCourse(ctx, req)
	if err != nil {
		return nil, err
	}

	// Non-nil because the RPC is declared authenticated in services.go;
	// authorizeUserForStreamCourse alone would return no user on a public course.
	progress := model.StreamProgress{
		StreamID: stream.ID,
		UserID:   user.ID,
		Progress: float64(req.Progress),
		Watched:  req.Watched,
	}

	err = a.dao.SaveProgresses([]model.StreamProgress{progress}) // Only support one progress update at a time for now
	if err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}

	return h.ParseStreamProgressToProto(progress), nil
}
