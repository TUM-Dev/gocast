package apiv2

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"go.uber.org/mock/gomock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	protobuf "github.com/TUM-Dev/gocast/apiv2/protobuf/server"
	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/mock_dao"
	"github.com/TUM-Dev/gocast/model"
	"github.com/TUM-Dev/gocast/tools"
)

// Liveness is derived rather than stored, so the page and the scheduler cannot
// disagree about it. Worth checking the derivation happens on the way out.
func TestListWorkersDerivesLiveness(t *testing.T) {
	now := time.Now()
	workers := []model.Worker{
		{WorkerID: "fresh", LastSeen: now.Add(-time.Second), Workload: 3, Version: "1.2.3"},
		{WorkerID: "stale", LastSeen: now.Add(-time.Hour), Workload: 0, Version: "1.0.0"},
		// On the boundary, which a client reimplementing the rule would get wrong.
		{WorkerID: "borderline", LastSeen: now.Add(-6 * time.Minute)},
	}

	ctrl := gomock.NewController(t)
	workerMock := mock_dao.NewMockWorkerDao(ctrl)
	workerMock.EXPECT().GetAllWorkers().Return(workers, nil).Times(1)

	tools.Cfg.WorkerToken = "the-worker-token"
	api := &API{dao: dao.DaoWrapper{WorkerDao: workerMock}, log: slog.Default()}

	resp, err := api.ListWorkers(context.Background(), &emptypb.Empty{})
	if err != nil {
		t.Fatalf("ListWorkers: %v", err)
	}

	if resp.WorkerToken != "the-worker-token" {
		t.Errorf("WorkerToken = %q, want %q", resp.WorkerToken, "the-worker-token")
	}

	alive := map[string]bool{}
	for _, worker := range resp.Workers {
		alive[worker.WorkerId] = worker.Alive
	}

	want := map[string]bool{"fresh": true, "stale": false, "borderline": false}
	for workerID, expected := range want {
		if alive[workerID] != expected {
			t.Errorf("%s: alive = %v, want %v", workerID, alive[workerID], expected)
		}
	}
}

// No workers registered is an ordinary state, not a failure.
func TestListWorkersWithNoneRegistered(t *testing.T) {
	ctrl := gomock.NewController(t)
	workerMock := mock_dao.NewMockWorkerDao(ctrl)
	workerMock.EXPECT().GetAllWorkers().Return([]model.Worker{}, nil).Times(1)

	api := &API{dao: dao.DaoWrapper{WorkerDao: workerMock}, log: slog.Default()}

	resp, err := api.ListWorkers(context.Background(), &emptypb.Empty{})
	if err != nil {
		t.Fatalf("ListWorkers: %v", err)
	}
	if len(resp.Workers) != 0 {
		t.Errorf("got %d workers, want none", len(resp.Workers))
	}
}

func TestDeleteWorker(t *testing.T) {
	t.Run("deletes the worker it was given", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		workerMock := mock_dao.NewMockWorkerDao(ctrl)
		workerMock.EXPECT().DeleteWorker("worker-1").Return(nil).Times(1)

		api := &API{dao: dao.DaoWrapper{WorkerDao: workerMock}, log: slog.Default()}

		if _, err := api.DeleteWorker(context.Background(), &protobuf.DeleteWorkerRequest{
			WorkerId: "worker-1",
		}); err != nil {
			t.Fatalf("DeleteWorker: %v", err)
		}
	})

	// gorm reads a Delete with no condition as the whole table.
	t.Run("refuses an empty worker id without reaching the database", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		workerMock := mock_dao.NewMockWorkerDao(ctrl)
		workerMock.EXPECT().DeleteWorker(gomock.Any()).Times(0)

		api := &API{dao: dao.DaoWrapper{WorkerDao: workerMock}, log: slog.Default()}

		_, err := api.DeleteWorker(context.Background(), &protobuf.DeleteWorkerRequest{})

		if got := status.Code(err); got != codes.InvalidArgument {
			t.Errorf("code = %v, want %v", got, codes.InvalidArgument)
		}
	})

	t.Run("reports a failed delete", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		workerMock := mock_dao.NewMockWorkerDao(ctrl)
		workerMock.EXPECT().DeleteWorker(gomock.Any()).
			Return(errors.New("database is on fire")).Times(1)

		api := &API{dao: dao.DaoWrapper{WorkerDao: workerMock}, log: slog.Default()}

		_, err := api.DeleteWorker(context.Background(), &protobuf.DeleteWorkerRequest{
			WorkerId: "worker-1",
		})
		if err == nil {
			t.Fatal("a failed delete was reported as success")
		}
	})
}
