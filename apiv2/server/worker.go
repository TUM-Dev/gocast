package apiv2

import (
	"context"
	"errors"
	"net/http"

	"google.golang.org/protobuf/types/known/emptypb"

	e "github.com/TUM-Dev/gocast/apiv2/errors"
	h "github.com/TUM-Dev/gocast/apiv2/helpers"
	protobuf "github.com/TUM-Dev/gocast/apiv2/protobuf/server"
	"github.com/TUM-Dev/gocast/tools"
)

// The worker_id is the primary key, and gorm reads a Delete with no condition as
// the whole table.
var errNoWorkerID = errors.New("no worker id given")

// ListWorkers returns every registered worker, plus the token a new worker
// authenticates its registration with. Both endpoints here are gated on
// PermAdministerServer by their policy in services.go.
func (a *API) ListWorkers(ctx context.Context, req *emptypb.Empty) (*protobuf.ListWorkersResponse, error) {
	workers, err := a.dao.WorkerDao.GetAllWorkers()
	if err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}

	out := make([]*protobuf.Worker, 0, len(workers))
	for _, worker := range workers {
		out = append(out, h.ParseWorkerToProto(worker))
	}

	return &protobuf.ListWorkersResponse{Workers: out, WorkerToken: tools.Cfg.WorkerToken}, nil
}

// DeleteWorker removes a worker's registration. Not a way to stop one: a worker
// still running re-registers on its next heartbeat.
func (a *API) DeleteWorker(ctx context.Context, req *protobuf.DeleteWorkerRequest) (*emptypb.Empty, error) {
	if req.GetWorkerId() == "" {
		return nil, e.WithStatus(http.StatusBadRequest, errNoWorkerID)
	}

	if err := a.dao.WorkerDao.DeleteWorker(req.GetWorkerId()); err != nil {
		return nil, e.FromGorm(err, "can't find worker")
	}

	return &emptypb.Empty{}, nil
}
