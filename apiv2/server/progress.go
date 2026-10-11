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
	}

	// req.Watched is nil for a plain progress report, which must not touch the flag.
	if err := a.dao.SaveProgress(progress, req.Watched); err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}

	// Answer with what is stored: a report that left the flag alone has no value of its own to echo.
	stored, err := a.dao.LoadProgress(user.ID, []uint{stream.ID})
	if err != nil || len(stored) == 0 {
		return nil, e.WithStatus(http.StatusInternalServerError, errors.New("could not read back the saved progress"))
	}

	return h.ParseStreamProgressToProto(stored[0]), nil
}
