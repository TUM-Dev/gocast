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
)

// Liveness is derived rather than stored, so the page and the scheduler cannot
// disagree about it. Worth checking the derivation happens on the way out.
func TestListRunnersDerivesLiveness(t *testing.T) {
	now := time.Now()
	runners := []model.Runner{
		{Hostname: "fresh", LastSeen: now.Add(-time.Second), JobCount: 3, Version: "1.2.3"},
		{Hostname: "stale", LastSeen: now.Add(-time.Hour), JobCount: 0, Version: "1.0.0"},
		// On the boundary, which a client reimplementing the rule would get wrong.
		{Hostname: "borderline", LastSeen: now.Add(-6 * time.Second)},
	}

	ctrl := gomock.NewController(t)
	runnerMock := mock_dao.NewMockRunnerDao(ctrl)
	runnerMock.EXPECT().GetAll(gomock.Any()).Return(runners, nil).Times(1)

	api := &API{dao: dao.DaoWrapper{RunnerDao: runnerMock}, log: slog.Default()}

	resp, err := api.ListRunners(context.Background(), &emptypb.Empty{})
	if err != nil {
		t.Fatalf("ListRunners: %v", err)
	}

	alive := map[string]bool{}
	for _, runner := range resp.Runners {
		alive[runner.Hostname] = runner.Alive
	}

	want := map[string]bool{"fresh": true, "stale": false, "borderline": false}
	for hostname, expected := range want {
		if alive[hostname] != expected {
			t.Errorf("%s: alive = %v, want %v", hostname, alive[hostname], expected)
		}
	}
}

// No runners registered is an ordinary state, not a failure.
func TestListRunnersWithNoneRegistered(t *testing.T) {
	ctrl := gomock.NewController(t)
	runnerMock := mock_dao.NewMockRunnerDao(ctrl)
	runnerMock.EXPECT().GetAll(gomock.Any()).Return([]model.Runner{}, nil).Times(1)

	api := &API{dao: dao.DaoWrapper{RunnerDao: runnerMock}, log: slog.Default()}

	resp, err := api.ListRunners(context.Background(), &emptypb.Empty{})
	if err != nil {
		t.Fatalf("ListRunners: %v", err)
	}
	if len(resp.Runners) != 0 {
		t.Errorf("got %d runners, want none", len(resp.Runners))
	}
}

func TestDeleteRunner(t *testing.T) {
	t.Run("deletes the runner it was given", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		runnerMock := mock_dao.NewMockRunnerDao(ctrl)
		runnerMock.EXPECT().Delete(gomock.Any(), "runner-1").Return(nil).Times(1)

		api := &API{dao: dao.DaoWrapper{RunnerDao: runnerMock}, log: slog.Default()}

		if _, err := api.DeleteRunner(context.Background(), &protobuf.DeleteRunnerRequest{
			Hostname: "runner-1",
		}); err != nil {
			t.Fatalf("DeleteRunner: %v", err)
		}
	})

	// gorm reads a Delete with no condition as the whole table.
	t.Run("refuses an empty hostname without reaching the database", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		runnerMock := mock_dao.NewMockRunnerDao(ctrl)
		runnerMock.EXPECT().Delete(gomock.Any(), gomock.Any()).Times(0)

		api := &API{dao: dao.DaoWrapper{RunnerDao: runnerMock}, log: slog.Default()}

		_, err := api.DeleteRunner(context.Background(), &protobuf.DeleteRunnerRequest{})

		if got := status.Code(err); got != codes.InvalidArgument {
			t.Errorf("code = %v, want %v", got, codes.InvalidArgument)
		}
	})

	t.Run("reports a failed delete", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		runnerMock := mock_dao.NewMockRunnerDao(ctrl)
		runnerMock.EXPECT().Delete(gomock.Any(), gomock.Any()).
			Return(errors.New("database is on fire")).Times(1)

		api := &API{dao: dao.DaoWrapper{RunnerDao: runnerMock}, log: slog.Default()}

		_, err := api.DeleteRunner(context.Background(), &protobuf.DeleteRunnerRequest{
			Hostname: "runner-1",
		})
		if err == nil {
			t.Fatal("a failed delete was reported as success")
		}
	})
}
