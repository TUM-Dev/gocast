package apiv2

import (
	"context"
	"errors"
	"net/http"

	"google.golang.org/protobuf/types/known/emptypb"

	e "github.com/TUM-Dev/gocast/apiv2/errors"
	h "github.com/TUM-Dev/gocast/apiv2/helpers"
	protobuf "github.com/TUM-Dev/gocast/apiv2/protobuf/server"
)

// The hostname is the primary key, and gorm reads a Delete with no condition as
// the whole table.
var errNoHostname = errors.New("no hostname given")

// ListRunners returns every registered runner. Both endpoints here are gated on
// PermAdministerServer by their policy in services.go.
func (a *API) ListRunners(ctx context.Context, req *emptypb.Empty) (*protobuf.ListRunnersResponse, error) {
	runners, err := a.dao.RunnerDao.GetAll(ctx)
	if err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}

	out := make([]*protobuf.Runner, 0, len(runners))
	for _, runner := range runners {
		out = append(out, h.ParseRunnerToProto(runner))
	}

	return &protobuf.ListRunnersResponse{Runners: out}, nil
}

// DeleteRunner removes a runner's registration. Not a way to stop one: a runner
// still running re-registers on its next heartbeat.
func (a *API) DeleteRunner(ctx context.Context, req *protobuf.DeleteRunnerRequest) (*emptypb.Empty, error) {
	if req.GetHostname() == "" {
		return nil, e.WithStatus(http.StatusBadRequest, errNoHostname)
	}

	if err := a.dao.RunnerDao.Delete(ctx, req.GetHostname()); err != nil {
		return nil, e.FromGorm(err, "can't find runner")
	}

	return &emptypb.Empty{}, nil
}
