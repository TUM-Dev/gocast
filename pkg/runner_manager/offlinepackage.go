package runner_manager

import (
	"context"
	"errors"
	"fmt"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/tum-dev/gocast/runner/pkg/ptr"
	"github.com/tum-dev/gocast/runner/protobuf"

	"github.com/TUM-Dev/gocast/model"
)

// ErrOfflineUnsupported is returned when the runner predates offline packages.
var ErrOfflineUnsupported = errors.New("runner can't make offline packages")

// offlineKeyURI names a package's key in its playlist. Nothing fetches it: the device
// gets the key with the licence and decrypts the segments itself.
func offlineKeyURI(packageID uint) string {
	return fmt.Sprintf("gocast-offline:%d", packageID)
}

// RequestOfflinePackage asks a runner to package a VOD for offline viewing. The runner
// answers asynchronously with an OfflinePackageReady or OfflinePackageFailed
// notification, so this returns as soon as the job is accepted.
func (m *Manager) RequestOfflinePackage(ctx context.Context, p model.OfflinePackage) error {
	runner, client, conn, err := m.getClient(ctx)
	if err != nil {
		return fmt.Errorf("getClient: %w", err)
	}
	defer func() {
		_ = conn.Close()
	}()

	ctx, cancel := context.WithTimeout(ctx, runnerDispatchTimeout)
	defer cancel()

	resp, err := client.RequestOfflinePackage(ctx, &protobuf.OfflinePackageRequest{
		PackageId: ptr.Take(uint64(p.ID)),
		StreamId:  ptr.Take(uint64(p.StreamID)),
		Version:   ptr.Take(protobufStreamVersion(p.Version)),
		Key:       p.Key,
		KeyUri:    ptr.Take(offlineKeyURI(p.ID)),
	})
	if status.Code(err) == codes.Unimplemented {
		return ErrOfflineUnsupported
	}
	if err != nil {
		return fmt.Errorf("RequestOfflinePackage: %w", err)
	}
	m.logger.With("package", p.ID, "stream", p.StreamID, "job", resp.GetJobId(), "runner", runner.Hostname).
		Info("requested offline package")
	return nil
}

// DeleteOfflinePackage asks a runner to remove a package the device has downloaded. Any
// runner will do: they share the storage the packages are in. A failure leaves the
// package to the runners' own cleanup, which removes it within a day.
func (m *Manager) DeleteOfflinePackage(ctx context.Context, packageID uint) error {
	_, client, conn, err := m.getClient(ctx)
	if err != nil {
		return fmt.Errorf("getClient: %w", err)
	}
	defer func() {
		_ = conn.Close()
	}()

	ctx, cancel := context.WithTimeout(ctx, runnerDispatchTimeout)
	defer cancel()

	if _, err := client.DeleteOfflinePackage(ctx, &protobuf.DeleteOfflinePackageRequest{
		PackageId: ptr.Take(uint64(packageID)),
	}); err != nil {
		return fmt.Errorf("DeleteOfflinePackage: %w", err)
	}
	return nil
}

const (
	// offlinePreparingMax is how long packaging may take before the job counts as lost:
	// a copy without re-encoding takes minutes even for a long lecture, so this only
	// trips when the runner died or its notification never came.
	offlinePreparingMax = 2 * time.Hour
	// offlineReadyMax matches the runners' own cleanup (offlinePackageMaxAge), which
	// removes a package that was never downloaded after a day.
	offlineReadyMax = 24 * time.Hour

	offlineLostReason = "Preparing the lecture took too long. Please try again."
	offlineAbandoned  = "The download wasn't finished in time. Please save the lecture again."
)

// ExpireOfflinePackages marks packages failed that will never become ready, or whose
// download was abandoned, so they stop counting against the limits on waiting packages.
func (m *Manager) ExpireOfflinePackages() {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	now := time.Now()
	if n, err := m.dao.OfflinePackageDao.FailStalePreparing(ctx, now.Add(-offlinePreparingMax), offlineLostReason); err != nil {
		m.logger.Error("can't expire offline packages being prepared", "err", err)
	} else if n > 0 {
		m.logger.Warn("offline packages were never reported ready", "count", n)
	}
	if _, err := m.dao.OfflinePackageDao.ExpireStaleReady(ctx, now.Add(-offlineReadyMax), offlineAbandoned); err != nil {
		m.logger.Error("can't expire abandoned offline packages", "err", err)
	}
}

func (m *Manager) handleOfflinePackageReady(ctx context.Context, n *protobuf.OfflinePackageReadyNotification) (*protobuf.NotificationResponse, error) {
	p, err := m.dao.OfflinePackageDao.Get(ctx, uint(n.GetPackageId()))
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "can't find offline package %d: %v", n.GetPackageId(), err)
	}
	// A notification retried after the device already moved on must not undo that.
	if p.State != model.OfflinePreparing {
		return &protobuf.NotificationResponse{}, nil
	}
	p.State = model.OfflineReady
	p.SegmentCount = n.GetSegmentCount()
	p.SizeBytes = n.GetSizeBytes()
	if err := m.dao.OfflinePackageDao.Update(ctx, &p); err != nil {
		return nil, status.Errorf(codes.Internal, "update offline package: %v", err)
	}
	return &protobuf.NotificationResponse{}, nil
}

func (m *Manager) handleOfflinePackageFailed(ctx context.Context, n *protobuf.OfflinePackageFailedNotification) (*protobuf.NotificationResponse, error) {
	p, err := m.dao.OfflinePackageDao.Get(ctx, uint(n.GetPackageId()))
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "can't find offline package %d: %v", n.GetPackageId(), err)
	}
	if p.State != model.OfflinePreparing {
		return &protobuf.NotificationResponse{}, nil
	}
	p.State = model.OfflineFailed
	p.Error = n.GetReason()
	if err := m.dao.OfflinePackageDao.Update(ctx, &p); err != nil {
		return nil, status.Errorf(codes.Internal, "update offline package: %v", err)
	}
	return &protobuf.NotificationResponse{}, nil
}
