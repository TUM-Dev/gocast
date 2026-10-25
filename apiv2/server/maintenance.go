package apiv2

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"

	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	e "github.com/TUM-Dev/gocast/apiv2/errors"
	protobuf "github.com/TUM-Dev/gocast/apiv2/protobuf/server"
	"github.com/TUM-Dev/gocast/model"
	"github.com/TUM-Dev/gocast/tools"
)

// Every RPC here is gated on PermAdministerServer by its policy in services.go.

// ListMaintenanceCronJobs lists the jobs the scheduler knows by name, for the page to
// offer as choices before triggering one manually.
func (a *API) ListMaintenanceCronJobs(ctx context.Context, req *emptypb.Empty) (*protobuf.ListMaintenanceCronJobsResponse, error) {
	return &protobuf.ListMaintenanceCronJobsResponse{Jobs: tools.Cron.ListCronJobs()}, nil
}

// RunMaintenanceCronJob executes a registered job immediately. Unlike the v1 route,
// which silently ignored a name it did not recognize, this refuses one — the page
// only ever offers names from ListMaintenanceCronJobs, so a mismatch means the
// scheduler changed underneath an open tab, and the caller should be told rather than
// shown a false success.
func (a *API) RunMaintenanceCronJob(ctx context.Context, req *protobuf.RunMaintenanceCronJobRequest) (*emptypb.Empty, error) {
	job := req.GetJob()
	if !slices.Contains(tools.Cron.ListCronJobs(), job) {
		return nil, e.WithStatus(http.StatusNotFound, errors.New("no such cron job"))
	}

	tools.Cron.RunJob(job)
	a.audit(ctx, model.AuditMaintenance, fmt.Sprintf("cron job '%s' run", job))
	return &emptypb.Empty{}, nil
}

// ListMaintenanceTranscodingFailures lists every recorded transcoding failure.
func (a *API) ListMaintenanceTranscodingFailures(ctx context.Context, req *emptypb.Empty) (*protobuf.ListMaintenanceTranscodingFailuresResponse, error) {
	failures, err := a.dao.TranscodingFailureDao.All()
	if err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}

	out := make([]*protobuf.MaintenanceTranscodingFailure, 0, len(failures))
	for _, failure := range failures {
		out = append(out, transcodingFailureMessage(failure))
	}

	return &protobuf.ListMaintenanceTranscodingFailuresResponse{Failures: out}, nil
}

// DeleteMaintenanceTranscodingFailure dismisses a recorded failure. It does not
// retry the transcode; a stream stuck this way is fixed the way it always has been.
func (a *API) DeleteMaintenanceTranscodingFailure(ctx context.Context, req *protobuf.DeleteMaintenanceTranscodingFailureRequest) (*emptypb.Empty, error) {
	if err := a.dao.TranscodingFailureDao.Delete(uint(req.GetId())); err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}
	a.audit(ctx, model.AuditMaintenance, fmt.Sprintf("transcoding failure %d dismissed", req.GetId()))
	return &emptypb.Empty{}, nil
}

func transcodingFailureMessage(failure model.TranscodingFailure) *protobuf.MaintenanceTranscodingFailure {
	return &protobuf.MaintenanceTranscodingFailure{
		Id:           uint32(failure.ID),
		StreamId:     uint32(failure.StreamID),
		Version:      string(failure.Version),
		FriendlyTime: failure.FriendlyTime,
		Hostname:     failure.Hostname,
		FilePath:     failure.FilePath,
		Logs:         failure.Logs,
	}
}

// ListMaintenanceEmailFailures lists every email that has exhausted its retries or is
// still failing, so an administrator can see why and dismiss the ones that are stale.
func (a *API) ListMaintenanceEmailFailures(ctx context.Context, req *emptypb.Empty) (*protobuf.ListMaintenanceEmailFailuresResponse, error) {
	failures, err := a.dao.EmailDao.GetFailed(ctx)
	if err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}

	out := make([]*protobuf.MaintenanceEmailFailure, 0, len(failures))
	for _, failure := range failures {
		out = append(out, emailFailureMessage(failure))
	}

	return &protobuf.ListMaintenanceEmailFailuresResponse{Failures: out}, nil
}

// DeleteMaintenanceEmailFailure dismisses a failed email. It is not retried afterwards.
func (a *API) DeleteMaintenanceEmailFailure(ctx context.Context, req *protobuf.DeleteMaintenanceEmailFailureRequest) (*emptypb.Empty, error) {
	if err := a.dao.EmailDao.Delete(ctx, uint(req.GetId())); err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}
	a.audit(ctx, model.AuditMaintenance, fmt.Sprintf("email failure %d dismissed", req.GetId()))
	return &emptypb.Empty{}, nil
}

func emailFailureMessage(failure model.Email) *protobuf.MaintenanceEmailFailure {
	msg := &protobuf.MaintenanceEmailFailure{
		Id:      uint32(failure.ID),
		To:      failure.To,
		Subject: failure.Subject,
		Body:    failure.Body,
		Retries: int32(failure.Retries),
		Errors:  failure.Errors,
	}
	// Zero when no attempt has been made yet; leaving LastTry unset reads more
	// honestly on the wire than a timestamp for the year 1.
	if !failure.LastTry.IsZero() {
		msg.LastTry = timestamppb.New(failure.LastTry)
	}
	return msg
}
