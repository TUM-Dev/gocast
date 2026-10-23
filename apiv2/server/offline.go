package apiv2

import (
	"context"
	"crypto/rand"
	"errors"
	"net/http"
	"strconv"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
	"gorm.io/gorm"

	e "github.com/TUM-Dev/gocast/apiv2/errors"
	protobuf "github.com/TUM-Dev/gocast/apiv2/protobuf/server"
	"github.com/TUM-Dev/gocast/apiv2/visibility"
	"github.com/TUM-Dev/gocast/model"
	"github.com/TUM-Dev/gocast/tools"
)

// Saving lectures for offline viewing. A runner packages the recording, encrypted with
// a key made for this one request; the device downloads the package, keeps the key,
// and has to come back within offlineLicenseTTL to renew the licence, which is when a
// lecture the viewer may no longer have is revoked.

// OfflinePackager reaches the runners that make and delete packages.
type OfflinePackager interface {
	RequestOfflinePackage(ctx context.Context, p model.OfflinePackage) error
	DeleteOfflinePackage(ctx context.Context, packageID uint) error
}

// WithOfflinePackager gives the API the runners to package lectures with. Without one,
// requestOfflineLecture answers 503.
func WithOfflinePackager(p OfflinePackager) Option {
	return func(a *API) { a.offline = p }
}

const (
	// offlineLicenseTTL is how long a saved lecture plays without the device renewing.
	offlineLicenseTTL = 7 * 24 * time.Hour

	// Packages are made on request and take up disk until the device has them, so the
	// limits are about disk as much as about bulk downloading: requests per viewer and
	// day, packages waiting per viewer, and packages waiting on the server.
	offlineRequestsPerDay = 20
	offlineActivePerUser  = 2
	offlineActiveTotal    = 50

	// offlineSyncMax bounds one sync request: a device holding more is not one person's.
	offlineSyncMax = 50

	offlineUnavailable = "Offline saving isn't available right now. Please try again later."
)

// offlineVersions maps the version a package was made from to the view it shows.
var offlineVersions = map[model.StreamVersion]protobuf.WatchViewKind{
	model.COMB: protobuf.WatchViewKind_WATCH_VIEW_KIND_COMBINED,
	model.PRES: protobuf.WatchViewKind_WATCH_VIEW_KIND_PRESENTATION,
	model.CAM:  protobuf.WatchViewKind_WATCH_VIEW_KIND_CAMERA,
}

// offlineSource picks what to package: the first view the watch page offers, so the
// course's source modes apply as they do there.
func offlineSource(s *model.Stream, c *model.Course) (watchSource, bool) {
	sources := watchSources(s, c)
	if len(sources) == 0 {
		return watchSource{}, false
	}
	return sources[0], true
}

// vodPlaylist is the unsigned playlist of one version of a recording.
func vodPlaylist(s *model.Stream, version model.StreamVersion) string {
	switch version {
	case model.PRES:
		return s.PlaylistUrlPRES
	case model.CAM:
		return s.PlaylistUrlCAM
	default:
		return s.PlaylistUrl
	}
}

func (a *API) RequestOfflineLecture(ctx context.Context, req *protobuf.RequestOfflineLectureRequest) (*protobuf.OfflineLecture, error) {
	if a.offline == nil {
		return nil, e.WithStatus(http.StatusServiceUnavailable, errors.New(offlineUnavailable))
	}
	user, stream, course, err := a.authorizeUserForStreamCourse(ctx, req)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, e.WithStatus(http.StatusUnauthorized, errors.New("sign in to save lectures offline"))
	}
	if !canDownloadStream(user, &stream, &course) {
		return nil, e.WithStatus(http.StatusForbidden, errors.New("this lecture can't be saved offline"))
	}
	src, ok := offlineSource(&stream, &course)
	if !ok {
		return nil, e.WithStatus(http.StatusForbidden, errors.New("this lecture has no recording to save"))
	}

	now := time.Now()
	if n, err := a.dao.OfflinePackageDao.CountSince(ctx, user.ID, now.Add(-24*time.Hour)); err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	} else if n >= offlineRequestsPerDay {
		return nil, e.WithStatus(http.StatusTooManyRequests, errors.New("you saved many lectures today; please try again tomorrow"))
	}
	if n, err := a.dao.OfflinePackageDao.CountActive(ctx, user.ID); err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	} else if n >= offlineActivePerUser {
		return nil, e.WithStatus(http.StatusTooManyRequests, errors.New("wait until the lectures you are saving have finished"))
	}
	if n, err := a.dao.OfflinePackageDao.CountActive(ctx, 0); err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	} else if n >= offlineActiveTotal {
		return nil, e.WithStatus(http.StatusServiceUnavailable, errors.New(offlineUnavailable))
	}

	key := make([]byte, 16)
	if _, err := rand.Read(key); err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}
	p := model.OfflinePackage{
		UserID:   user.ID,
		StreamID: stream.ID,
		Version:  model.StreamVersion(src.thumb),
		State:    model.OfflinePreparing,
		Key:      key,
	}
	if err := a.dao.OfflinePackageDao.Create(ctx, &p); err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}
	if err := a.offline.RequestOfflinePackage(ctx, p); err != nil {
		a.log.Error("can't request offline package", "err", err, "package", p.ID, "stream", stream.ID)
		p.State = model.OfflineFailed
		p.Error = offlineUnavailable
		if err := a.dao.OfflinePackageDao.Update(ctx, &p); err != nil {
			a.log.Error("can't mark offline package failed", "err", err, "package", p.ID)
		}
		return nil, e.WithStatus(http.StatusServiceUnavailable, errors.New(offlineUnavailable))
	}
	return offlineLecture(&p, &stream, &course, nil), nil
}

func (a *API) GetOfflineLecture(ctx context.Context, req *protobuf.GetOfflineLectureRequest) (*protobuf.OfflineLecture, error) {
	user, p, err := a.ownOfflinePackage(ctx, req.GetPackageId())
	if err != nil {
		return nil, err
	}
	stream, course, err := a.offlineStreamCourse(ctx, p.StreamID)
	if err != nil {
		return nil, err
	}
	if p.State != model.OfflineReady {
		return offlineLecture(&p, &stream, &course, nil), nil
	}

	// The key leaves the server here, so whether the viewer may have it is decided
	// again: the course may have stopped allowing downloads since the request.
	if !a.mayKeepOffline(user, &stream, &course) {
		a.revokeOffline(ctx, &p)
		return offlineLecture(&p, &stream, &course, nil), nil
	}
	playlist, err := tools.OfflinePlaylistURL(vodPlaylist(&stream, p.Version), p.ID, &stream, user)
	if err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}
	p.ExpiresAt = time.Now().Add(offlineLicenseTTL)
	if err := a.dao.OfflinePackageDao.Update(ctx, &p); err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}
	return offlineLecture(&p, &stream, &course, &protobuf.OfflineLicense{
		Key:         p.Key,
		ExpiresAt:   timestamppb.New(p.ExpiresAt),
		PlaylistUrl: playlist,
	}), nil
}

func (a *API) CompleteOfflineDownload(ctx context.Context, req *protobuf.CompleteOfflineDownloadRequest) (*emptypb.Empty, error) {
	_, p, err := a.ownOfflinePackage(ctx, req.GetPackageId())
	if err != nil {
		return nil, err
	}
	switch p.State {
	case model.OfflineDownloaded:
		// A retry after a lost answer.
		return &emptypb.Empty{}, nil
	case model.OfflineReady:
	default:
		return nil, e.WithStatus(http.StatusConflict, errors.New("the lecture isn't ready to download"))
	}
	p.State = model.OfflineDownloaded
	if err := a.dao.OfflinePackageDao.Update(ctx, &p); err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}
	a.deleteOfflinePackage(ctx, p.ID)
	return &emptypb.Empty{}, nil
}

func (a *API) DeleteOfflineLecture(ctx context.Context, req *protobuf.DeleteOfflineLectureRequest) (*emptypb.Empty, error) {
	_, p, err := a.ownOfflinePackage(ctx, req.GetPackageId())
	if err != nil {
		return nil, err
	}
	if p.State != model.OfflineRevoked {
		a.revokeOffline(ctx, &p)
	}
	return &emptypb.Empty{}, nil
}

func (a *API) SyncOfflineLicenses(ctx context.Context, req *protobuf.SyncOfflineLicensesRequest) (*protobuf.SyncOfflineLicensesResponse, error) {
	user, err := a.getCurrent(ctx)
	if err != nil {
		return nil, e.WithStatus(http.StatusUnauthorized, err)
	}
	if len(req.GetPackageIds()) > offlineSyncMax {
		return nil, e.WithStatus(http.StatusBadRequest, errors.New("too many packages"))
	}
	ids := make([]uint, 0, len(req.GetPackageIds()))
	for _, id := range req.GetPackageIds() {
		ids = append(ids, uint(id))
	}
	packages, err := a.dao.OfflinePackageDao.ListForUser(ctx, user.ID, ids)
	if err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}
	byID := make(map[uint]model.OfflinePackage, len(packages))
	for _, p := range packages {
		byID[p.ID] = p
	}

	// A device usually holds several lectures of one course; each stream is loaded
	// once, and a stream that fails to load is answered as renewable next time rather
	// than revoked: a database hiccup must not delete a student's lectures.
	type loaded struct {
		stream model.Stream
		course model.Course
		err    error
	}
	streams := map[uint]loaded{}
	resp := &protobuf.SyncOfflineLicensesResponse{}
	now := time.Now()
	for _, id := range ids {
		license := &protobuf.OfflineLicenseStatus{PackageId: uint32(id), Revoked: true}
		resp.Licenses = append(resp.Licenses, license)

		p, ok := byID[id]
		if !ok || (p.State != model.OfflineDownloaded && p.State != model.OfflineReady) {
			continue
		}
		l, ok := streams[p.StreamID]
		if !ok {
			l.stream, l.course, l.err = a.offlineStreamCourse(ctx, p.StreamID)
			streams[p.StreamID] = l
		}
		if l.err != nil && status.Code(l.err) != codes.NotFound {
			a.log.Error("can't load stream to renew an offline licence", "err", l.err, "stream", p.StreamID)
			license.Revoked = false
			license.ExpiresAt = timestamppb.New(p.ExpiresAt)
			continue
		}
		if l.err != nil || !a.mayKeepOffline(user, &l.stream, &l.course) {
			a.revokeOffline(ctx, &p)
			continue
		}
		p.ExpiresAt = now.Add(offlineLicenseTTL)
		if err := a.dao.OfflinePackageDao.Update(ctx, &p); err != nil {
			return nil, e.WithStatus(http.StatusInternalServerError, err)
		}
		license.Revoked = false
		license.ExpiresAt = timestamppb.New(p.ExpiresAt)
	}
	return resp, nil
}

// mayKeepOffline is authorizeUserForStreamCourse's check plus canDownloadStream, for a
// stream the caller named by a package rather than in the request.
func (a *API) mayKeepOffline(user *model.User, stream *model.Stream, course *model.Course) bool {
	return user.IsEligibleToWatchCourse(*course) &&
		visibility.StreamVisible(user, *course, *stream) &&
		canDownloadStream(user, stream, course)
}

// ownOfflinePackage returns the caller and one of the caller's packages. Another
// caller's package is a 404, as if it did not exist.
func (a *API) ownOfflinePackage(ctx context.Context, id uint32) (*model.User, model.OfflinePackage, error) {
	user, err := a.getCurrent(ctx)
	if err != nil {
		return nil, model.OfflinePackage{}, e.WithStatus(http.StatusUnauthorized, err)
	}
	p, err := a.dao.OfflinePackageDao.Get(ctx, uint(id))
	if err != nil {
		return nil, p, e.FromGorm(err, "can't find the saved lecture")
	}
	if p.UserID != user.ID {
		return nil, model.OfflinePackage{}, e.WithStatus(http.StatusNotFound, errors.New("can't find the saved lecture"))
	}
	return user, p, nil
}

// errOfflineStreamGone means a package's lecture was deleted: its licence is revoked.
// It reaches callers as a NotFound status, which is what SyncOfflineLicenses checks.
var errOfflineStreamGone = errors.New("the saved lecture no longer exists")

func (a *API) offlineStreamCourse(ctx context.Context, streamID uint) (model.Stream, model.Course, error) {
	stream, err := a.dao.GetStreamByID(ctx, strconv.FormatUint(uint64(streamID), 10))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return stream, model.Course{}, e.WithStatus(http.StatusNotFound, errOfflineStreamGone)
		}
		return stream, model.Course{}, e.WithStatus(http.StatusInternalServerError, err)
	}
	course, err := a.dao.GetCourseById(ctx, stream.CourseID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return stream, course, e.WithStatus(http.StatusNotFound, errOfflineStreamGone)
		}
		return stream, course, e.WithStatus(http.StatusInternalServerError, err)
	}
	return stream, course, nil
}

// revokeOffline ends a package's licence and removes it from the server if it is still
// there. Failures are logged only: the next sync revokes it again, and the runners
// remove a forgotten package within a day.
func (a *API) revokeOffline(ctx context.Context, p *model.OfflinePackage) {
	wasOnServer := p.Active()
	p.State = model.OfflineRevoked
	if err := a.dao.OfflinePackageDao.Update(ctx, p); err != nil {
		a.log.Error("can't revoke offline package", "err", err, "package", p.ID)
	}
	if wasOnServer {
		a.deleteOfflinePackage(ctx, p.ID)
	}
}

func (a *API) deleteOfflinePackage(ctx context.Context, id uint) {
	if a.offline == nil {
		return
	}
	if err := a.offline.DeleteOfflinePackage(ctx, id); err != nil {
		a.log.Warn("can't delete offline package, the runners will", "err", err, "package", id)
	}
}

var offlineStates = map[model.OfflinePackageState]protobuf.OfflineLectureState{
	model.OfflinePreparing:  protobuf.OfflineLectureState_OFFLINE_LECTURE_STATE_PREPARING,
	model.OfflineReady:      protobuf.OfflineLectureState_OFFLINE_LECTURE_STATE_READY,
	model.OfflineDownloaded: protobuf.OfflineLectureState_OFFLINE_LECTURE_STATE_DOWNLOADED,
	model.OfflineFailed:     protobuf.OfflineLectureState_OFFLINE_LECTURE_STATE_FAILED,
	model.OfflineRevoked:    protobuf.OfflineLectureState_OFFLINE_LECTURE_STATE_REVOKED,
}

func offlineLecture(p *model.OfflinePackage, s *model.Stream, c *model.Course, license *protobuf.OfflineLicense) *protobuf.OfflineLecture {
	out := &protobuf.OfflineLecture{
		PackageId:    uint32(p.ID),
		State:        offlineStates[p.State],
		Stream:       watchStream(s, nil),
		Course:       watchCourse(c),
		Kind:         offlineVersions[p.Version],
		SegmentCount: p.SegmentCount,
		SizeBytes:    p.SizeBytes,
		License:      license,
	}
	if p.Error != "" {
		out.Error = &p.Error
	}
	if start := uint32(s.StartOffset / 1000); start > 0 {
		out.StartOffset = &start
	}
	if end := uint32(s.EndOffset / 1000); end > 0 {
		out.EndOffset = &end
	}
	return out
}
