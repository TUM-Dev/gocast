package apiv2

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"sync"

	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/TUM-Dev/gocast/api"
	e "github.com/TUM-Dev/gocast/apiv2/errors"
	protobuf "github.com/TUM-Dev/gocast/apiv2/protobuf/server"
	"github.com/TUM-Dev/gocast/model"
	"github.com/TUM-Dev/gocast/tools"
)

// Every RPC here is gated on PermAdministerServer by its policy in services.go.

// maintenanceThumbnails tracks the one thumbnail-regeneration run that can be in
// flight at a time. Package-level rather than on API, the way tools.Cron is a global:
// there is exactly one of these per process regardless of how many API instances a
// test constructs.
var maintenanceThumbnails = struct {
	mu       sync.Mutex
	running  bool
	progress float32
}{}

// GetMaintenanceThumbnailStatus reports the state of the background regeneration job,
// for the page to poll instead of blocking on a request that can run for a long time.
func (a *API) GetMaintenanceThumbnailStatus(ctx context.Context, req *emptypb.Empty) (*protobuf.MaintenanceThumbnailStatus, error) {
	maintenanceThumbnails.mu.Lock()
	defer maintenanceThumbnails.mu.Unlock()

	return &protobuf.MaintenanceThumbnailStatus{
		Running:  maintenanceThumbnails.running,
		Progress: maintenanceThumbnails.progress,
	}, nil
}

// GenerateMaintenanceThumbnails starts a background job that requests a fresh
// thumbnail for every VoD file. A run already in progress is left alone: the old page
// disabled its button while running, but nothing stops two admins opening the page
// at once, so the handler itself has to be the guard.
func (a *API) GenerateMaintenanceThumbnails(ctx context.Context, req *emptypb.Empty) (*protobuf.MaintenanceThumbnailStatus, error) {
	maintenanceThumbnails.mu.Lock()
	if maintenanceThumbnails.running {
		defer maintenanceThumbnails.mu.Unlock()
		return &protobuf.MaintenanceThumbnailStatus{Running: true, Progress: maintenanceThumbnails.progress}, nil
	}

	noFiles, err := a.dao.FileDao.CountVoDFiles()
	if err != nil {
		maintenanceThumbnails.mu.Unlock()
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}

	// Set synchronously, before the goroutine starts, so the response this call
	// returns and the state a concurrent status poll sees can never disagree about
	// whether a run has begun.
	maintenanceThumbnails.running = true
	maintenanceThumbnails.progress = 0
	maintenanceThumbnails.mu.Unlock()

	daoWrapper := a.dao
	go func() {
		defer func() {
			maintenanceThumbnails.mu.Lock()
			maintenanceThumbnails.running = false
			maintenanceThumbnails.progress = 0
			maintenanceThumbnails.mu.Unlock()
		}()

		courses, err := daoWrapper.CoursesDao.GetAllCourses()
		if err != nil {
			a.log.Error("maintenance: can't get courses", "err", err)
			return
		}

		processed := 0
		// Iterate over every course. Some streams already have a valid thumbnail;
		// there is no cheap way to tell which without asking a worker, so this
		// regenerates all of them rather than trying to be clever about it.
		for _, course := range courses {
			for _, stream := range course.Streams {
				for _, file := range stream.Files {
					if file.Type != model.FILETYPE_VOD {
						continue
					}
					if err := api.RegenerateThumbs(daoWrapper, file, &stream, &course); err != nil {
						a.log.Error("maintenance: can't regenerate thumbnail",
							"streamID", stream.ID, "file", file.Path, "err", err)
						continue
					}
					processed++
					if noFiles > 0 {
						maintenanceThumbnails.mu.Lock()
						maintenanceThumbnails.progress = float32(processed) / float32(noFiles)
						maintenanceThumbnails.mu.Unlock()
					}
				}
			}
		}
	}()

	return &protobuf.MaintenanceThumbnailStatus{Running: true, Progress: 0}, nil
}

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
