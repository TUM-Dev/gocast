package apiv2

import (
	"context"
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

func TestListMaintenanceCronJobs(t *testing.T) {
	before := tools.Cron
	tools.InitCronService()
	t.Cleanup(func() { tools.Cron = before })

	if err := tools.Cron.AddFunc("fetchCourses", func() {}, "0 0 * * *"); err != nil {
		t.Fatalf("AddFunc: %v", err)
	}

	api := &API{log: slog.Default()}
	resp, err := api.ListMaintenanceCronJobs(context.Background(), &emptypb.Empty{})
	if err != nil {
		t.Fatalf("ListMaintenanceCronJobs: %v", err)
	}
	if len(resp.Jobs) != 1 || resp.Jobs[0] != "fetchCourses" {
		t.Errorf("Jobs = %v, want [fetchCourses]", resp.Jobs)
	}
}

func TestRunMaintenanceCronJob(t *testing.T) {
	t.Run("runs a job the scheduler knows about", func(t *testing.T) {
		before := tools.Cron
		tools.InitCronService()
		t.Cleanup(func() { tools.Cron = before })

		ran := make(chan struct{})
		if err := tools.Cron.AddFunc("collectStats", func() { close(ran) }, "0 0 * * *"); err != nil {
			t.Fatalf("AddFunc: %v", err)
		}

		api := &API{dao: dao.DaoWrapper{AuditDao: auditSink(t)}, log: slog.Default()}
		if _, err := api.RunMaintenanceCronJob(context.Background(), &protobuf.RunMaintenanceCronJobRequest{
			Job: "collectStats",
		}); err != nil {
			t.Fatalf("RunMaintenanceCronJob: %v", err)
		}
		<-ran
	})

	// The page only ever offers names ListMaintenanceCronJobs returned, so reaching
	// this means the scheduler changed underneath an open tab; the caller should be
	// told rather than shown a false success, unlike the v1 route this replaces.
	t.Run("refuses a name the scheduler does not know", func(t *testing.T) {
		before := tools.Cron
		tools.InitCronService()
		t.Cleanup(func() { tools.Cron = before })

		api := &API{log: slog.Default()}
		_, err := api.RunMaintenanceCronJob(context.Background(), &protobuf.RunMaintenanceCronJobRequest{
			Job: "nonexistent",
		})
		if got := status.Code(err); got != codes.NotFound {
			t.Errorf("code = %v, want %v", got, codes.NotFound)
		}
	})
}

func TestListMaintenanceTranscodingFailures(t *testing.T) {
	ctrl := gomock.NewController(t)
	failureMock := mock_dao.NewMockTranscodingFailureDao(ctrl)
	failureMock.EXPECT().All().Return([]model.TranscodingFailure{
		{
			StreamID:     1,
			Version:      model.COMB,
			Hostname:     "worker-1",
			FilePath:     "/rec/1.mp4",
			Logs:         "boom",
			FriendlyTime: "01.01.2026 00:00",
		},
	}, nil).Times(1)

	api := &API{dao: dao.DaoWrapper{TranscodingFailureDao: failureMock}, log: slog.Default()}
	resp, err := api.ListMaintenanceTranscodingFailures(context.Background(), &emptypb.Empty{})
	if err != nil {
		t.Fatalf("ListMaintenanceTranscodingFailures: %v", err)
	}
	if len(resp.Failures) != 1 {
		t.Fatalf("got %d failures, want 1", len(resp.Failures))
	}
	got := resp.Failures[0]
	if got.StreamId != 1 || got.Version != "COMB" || got.Hostname != "worker-1" {
		t.Errorf("failure = %+v, unexpected fields", got)
	}
}

func TestDeleteMaintenanceTranscodingFailure(t *testing.T) {
	ctrl := gomock.NewController(t)
	failureMock := mock_dao.NewMockTranscodingFailureDao(ctrl)
	failureMock.EXPECT().Delete(uint(7)).Return(nil).Times(1)

	api := &API{dao: dao.DaoWrapper{
		AuditDao:              auditSink(t),
		TranscodingFailureDao: failureMock}, log: slog.Default()}
	if _, err := api.DeleteMaintenanceTranscodingFailure(context.Background(), &protobuf.DeleteMaintenanceTranscodingFailureRequest{
		Id: 7,
	}); err != nil {
		t.Fatalf("DeleteMaintenanceTranscodingFailure: %v", err)
	}
}

func TestListMaintenanceEmailFailures(t *testing.T) {
	t.Run("carries every field including a last-try timestamp", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		emailMock := mock_dao.NewMockEmailDao(ctrl)
		lastTry := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
		emailMock.EXPECT().GetFailed(gomock.Any()).Return([]model.Email{
			{To: "a@b.de", Subject: "s", Body: "b", Retries: 3, Errors: "boom", LastTry: lastTry},
		}, nil).Times(1)

		api := &API{dao: dao.DaoWrapper{EmailDao: emailMock}, log: slog.Default()}
		resp, err := api.ListMaintenanceEmailFailures(context.Background(), &emptypb.Empty{})
		if err != nil {
			t.Fatalf("ListMaintenanceEmailFailures: %v", err)
		}
		if len(resp.Failures) != 1 {
			t.Fatalf("got %d failures, want 1", len(resp.Failures))
		}
		got := resp.Failures[0]
		if got.To != "a@b.de" || got.Retries != 3 || got.GetLastTry() == nil {
			t.Errorf("failure = %+v, unexpected fields", got)
		}
	})

	// A never-attempted send has a zero LastTry; sending it as a timestamp would read
	// as a real attempt in the year 1 rather than as "no attempt yet".
	t.Run("leaves last-try unset for an email that was never attempted", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		emailMock := mock_dao.NewMockEmailDao(ctrl)
		emailMock.EXPECT().GetFailed(gomock.Any()).Return([]model.Email{
			{To: "a@b.de"},
		}, nil).Times(1)

		api := &API{dao: dao.DaoWrapper{EmailDao: emailMock}, log: slog.Default()}
		resp, err := api.ListMaintenanceEmailFailures(context.Background(), &emptypb.Empty{})
		if err != nil {
			t.Fatalf("ListMaintenanceEmailFailures: %v", err)
		}
		if got := resp.Failures[0].GetLastTry(); got != nil {
			t.Errorf("LastTry = %v, want unset", got)
		}
	})
}

func TestDeleteMaintenanceEmailFailure(t *testing.T) {
	ctrl := gomock.NewController(t)
	emailMock := mock_dao.NewMockEmailDao(ctrl)
	emailMock.EXPECT().Delete(gomock.Any(), uint(9)).Return(nil).Times(1)

	api := &API{dao: dao.DaoWrapper{
		AuditDao: auditSink(t),
		EmailDao: emailMock}, log: slog.Default()}
	if _, err := api.DeleteMaintenanceEmailFailure(context.Background(), &protobuf.DeleteMaintenanceEmailFailureRequest{
		Id: 9,
	}); err != nil {
		t.Fatalf("DeleteMaintenanceEmailFailure: %v", err)
	}
}
