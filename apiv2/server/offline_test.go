package apiv2

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"go.uber.org/mock/gomock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	protobuf "github.com/TUM-Dev/gocast/apiv2/protobuf/server"
	"github.com/TUM-Dev/gocast/mock_dao"
	"github.com/TUM-Dev/gocast/model"
)

// fakePackager records what the handlers ask of the runners.
type fakePackager struct {
	requested []model.OfflinePackage
	deleted   []uint
	err       error
}

func (f *fakePackager) RequestOfflinePackage(_ context.Context, p model.OfflinePackage) error {
	f.requested = append(f.requested, p)
	return f.err
}

func (f *fakePackager) DeleteOfflinePackage(_ context.Context, id uint) error {
	f.deleted = append(f.deleted, id)
	return nil
}

type offlineEnv struct {
	*watchEnv
	packages *mock_dao.MockOfflinePackageDao
	packager *fakePackager
}

// newOfflineEnv is the watch page's stream 7, recorded, in a course that allows
// downloads, with its playlists where runners put them.
func newOfflineEnv(t *testing.T) *offlineEnv {
	t.Helper()
	env := &offlineEnv{watchEnv: newWatchEnv(t), packager: &fakePackager{}}
	env.course.DownloadsEnabled = true
	env.stream.PlaylistUrl = "https://edge.example/vod/7/STREAM_VERSION_COMBINED/playlist.m3u8"
	env.packages = mock_dao.NewMockOfflinePackageDao(gomock.NewController(t))
	env.api.dao.OfflinePackageDao = env.packages
	env.api.offline = env.packager
	return env
}

// underLimits lets a request past the rate limits.
func (env *offlineEnv) underLimits() {
	env.packages.EXPECT().CountSince(gomock.Any(), gomock.Any(), gomock.Any()).Return(int64(0), nil).AnyTimes()
	env.packages.EXPECT().CountActive(gomock.Any(), gomock.Any()).Return(int64(0), nil).AnyTimes()
}

func (env *offlineEnv) request(user *model.User) (*protobuf.OfflineLecture, error) {
	return env.api.RequestOfflineLecture(asCaller(user), &protobuf.RequestOfflineLectureRequest{StreamId: 7})
}

func TestRequestOfflineLectureFollowsTheDownloadRule(t *testing.T) {
	for _, c := range []struct {
		name      string
		downloads bool
		user      *model.User
		want      codes.Code
	}{
		{"course allows, student", true, student, codes.OK},
		{"course forbids, student", false, student, codes.PermissionDenied},
		{"course forbids, admin", false, courseAdm, codes.OK},
		{"anonymous", true, nil, codes.Unauthenticated},
	} {
		t.Run(c.name, func(t *testing.T) {
			env := newOfflineEnv(t)
			env.course.Visibility = "public"
			env.course.DownloadsEnabled = c.downloads
			env.underLimits()
			env.packages.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, p *model.OfflinePackage) error {
				p.ID = 9
				return nil
			}).AnyTimes()

			got, err := env.request(c.user)
			if status.Code(err) != c.want {
				t.Fatalf("RequestOfflineLecture = %v, want %v", err, c.want)
			}
			if c.want != codes.OK {
				if len(env.packager.requested) != 0 {
					t.Errorf("a package was requested: %v", env.packager.requested)
				}
				return
			}
			if got.State != protobuf.OfflineLectureState_OFFLINE_LECTURE_STATE_PREPARING || got.License != nil {
				t.Errorf("answer = %v, want preparing without a licence", got)
			}
			if len(env.packager.requested) != 1 {
				t.Fatalf("requested %d packages, want 1", len(env.packager.requested))
			}
			p := env.packager.requested[0]
			if p.ID != 9 || p.StreamID != 7 || p.Version != model.COMB || len(p.Key) != 16 {
				t.Errorf("requested %+v", p)
			}
		})
	}
}

func TestRequestOfflineLectureLimits(t *testing.T) {
	for _, c := range []struct {
		name                   string
		today, mine, everybody int64
		want                   codes.Code
	}{
		{"too many today", offlineRequestsPerDay, 0, 0, codes.ResourceExhausted},
		{"too many of mine waiting", 0, offlineActivePerUser, 0, codes.ResourceExhausted},
		{"server full", 0, 0, offlineActiveTotal, codes.Unavailable},
	} {
		t.Run(c.name, func(t *testing.T) {
			env := newOfflineEnv(t)
			env.packages.EXPECT().CountSince(gomock.Any(), student.ID, gomock.Any()).Return(c.today, nil).AnyTimes()
			env.packages.EXPECT().CountActive(gomock.Any(), student.ID).Return(c.mine, nil).AnyTimes()
			env.packages.EXPECT().CountActive(gomock.Any(), uint(0)).Return(c.everybody, nil).AnyTimes()

			if _, err := env.request(student); status.Code(err) != c.want {
				t.Errorf("RequestOfflineLecture = %v, want %v", err, c.want)
			}
		})
	}
}

func TestRequestOfflineLectureWithoutARunner(t *testing.T) {
	env := newOfflineEnv(t)
	env.packager.err = errors.New("no runner is available")
	env.underLimits()
	env.packages.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)
	env.packages.EXPECT().Update(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, p *model.OfflinePackage) error {
		if p.State != model.OfflineFailed {
			t.Errorf("package left %s", p.State)
		}
		return nil
	})

	if _, err := env.request(student); status.Code(err) != codes.Unavailable {
		t.Errorf("RequestOfflineLecture = %v, want Unavailable", err)
	}
}

func readyPackage() model.OfflinePackage {
	return model.OfflinePackage{
		ID: 9, UserID: student.ID, StreamID: 7, Version: model.COMB, State: model.OfflineReady,
		Key: []byte("0123456789abcdef"), SegmentCount: 3, SizeBytes: 1000,
	}
}

func TestGetOfflineLectureHandsOutTheLicence(t *testing.T) {
	env := newOfflineEnv(t)
	env.packages.EXPECT().Get(gomock.Any(), uint(9)).Return(readyPackage(), nil)
	env.packages.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)

	got, err := env.api.GetOfflineLecture(asCaller(student), &protobuf.GetOfflineLectureRequest{PackageId: 9})
	if err != nil {
		t.Fatalf("GetOfflineLecture = %v", err)
	}
	l := got.GetLicense()
	if string(l.GetKey()) != "0123456789abcdef" {
		t.Errorf("key = %q", l.GetKey())
	}
	if !strings.HasPrefix(l.GetPlaylistUrl(), "https://edge.example/vod/offline/9/playlist.m3u8?jwt=") {
		t.Errorf("playlist = %q", l.GetPlaylistUrl())
	}
	if d := time.Until(l.GetExpiresAt().AsTime()); d < offlineLicenseTTL-time.Minute || d > offlineLicenseTTL {
		t.Errorf("expires in %v, want %v", d, offlineLicenseTTL)
	}
	if got.Kind != protobuf.WatchViewKind_WATCH_VIEW_KIND_COMBINED || got.GetCourse().GetSlug() != "gbs" {
		t.Errorf("lecture = %v", got)
	}
}

func TestGetOfflineLectureRevokesWhenDownloadsWereSwitchedOff(t *testing.T) {
	env := newOfflineEnv(t)
	env.course.DownloadsEnabled = false
	env.packages.EXPECT().Get(gomock.Any(), uint(9)).Return(readyPackage(), nil)
	env.packages.EXPECT().Update(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, p *model.OfflinePackage) error {
		if p.State != model.OfflineRevoked {
			t.Errorf("package left %s", p.State)
		}
		return nil
	})

	got, err := env.api.GetOfflineLecture(asCaller(student), &protobuf.GetOfflineLectureRequest{PackageId: 9})
	if err != nil {
		t.Fatalf("GetOfflineLecture = %v", err)
	}
	if got.License != nil || got.State != protobuf.OfflineLectureState_OFFLINE_LECTURE_STATE_REVOKED {
		t.Errorf("answer = %v, want revoked without a licence", got)
	}
	if len(env.packager.deleted) != 1 {
		t.Errorf("deleted %v, want the package", env.packager.deleted)
	}
}

func TestOfflinePackagesAreTheirOwnersOnly(t *testing.T) {
	env := newOfflineEnv(t)
	p := readyPackage()
	p.UserID = outsider.ID
	env.packages.EXPECT().Get(gomock.Any(), uint(9)).Return(p, nil).Times(3)

	ctx := asCaller(student)
	_, err := env.api.GetOfflineLecture(ctx, &protobuf.GetOfflineLectureRequest{PackageId: 9})
	if status.Code(err) != codes.NotFound {
		t.Errorf("GetOfflineLecture = %v, want NotFound", err)
	}
	_, err = env.api.CompleteOfflineDownload(ctx, &protobuf.CompleteOfflineDownloadRequest{PackageId: 9})
	if status.Code(err) != codes.NotFound {
		t.Errorf("CompleteOfflineDownload = %v, want NotFound", err)
	}
	_, err = env.api.DeleteOfflineLecture(ctx, &protobuf.DeleteOfflineLectureRequest{PackageId: 9})
	if status.Code(err) != codes.NotFound {
		t.Errorf("DeleteOfflineLecture = %v, want NotFound", err)
	}
}

func TestCompleteOfflineDownloadDeletesThePackage(t *testing.T) {
	env := newOfflineEnv(t)
	env.packages.EXPECT().Get(gomock.Any(), uint(9)).Return(readyPackage(), nil)
	env.packages.EXPECT().Update(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, p *model.OfflinePackage) error {
		if p.State != model.OfflineDownloaded {
			t.Errorf("package left %s", p.State)
		}
		return nil
	})

	if _, err := env.api.CompleteOfflineDownload(asCaller(student), &protobuf.CompleteOfflineDownloadRequest{PackageId: 9}); err != nil {
		t.Fatalf("CompleteOfflineDownload = %v", err)
	}
	if len(env.packager.deleted) != 1 || env.packager.deleted[0] != 9 {
		t.Errorf("deleted %v, want [9]", env.packager.deleted)
	}
}

func TestSyncOfflineLicenses(t *testing.T) {
	env := newOfflineEnv(t)
	mine := readyPackage()
	mine.State = model.OfflineDownloaded
	gone := mine
	gone.ID = 10
	gone.StreamID = 8 // deleted since
	// 11 is another user's, so ListForUser leaves it out.
	env.packages.EXPECT().ListForUser(gomock.Any(), student.ID, []uint{9, 10, 11}).Return([]model.OfflinePackage{mine, gone}, nil)
	env.packages.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil).Times(2)

	resp, err := env.api.SyncOfflineLicenses(asCaller(student), &protobuf.SyncOfflineLicensesRequest{PackageIds: []uint32{9, 10, 11}})
	if err != nil {
		t.Fatalf("SyncOfflineLicenses = %v", err)
	}
	got := map[uint32]bool{}
	for _, l := range resp.Licenses {
		got[l.PackageId] = l.Revoked
		if !l.Revoked && time.Until(l.GetExpiresAt().AsTime()) < offlineLicenseTTL-time.Minute {
			t.Errorf("package %d renewed until %v", l.PackageId, l.GetExpiresAt().AsTime())
		}
	}
	if want := map[uint32]bool{9: false, 10: true, 11: true}; len(got) != 3 || got[9] != want[9] || got[10] != want[10] || got[11] != want[11] {
		t.Errorf("revoked = %v, want %v", got, want)
	}
}

func TestSyncOfflineLicensesRevokesWhenDownloadsWereSwitchedOff(t *testing.T) {
	env := newOfflineEnv(t)
	env.course.DownloadsEnabled = false
	p := readyPackage()
	p.State = model.OfflineDownloaded
	env.packages.EXPECT().ListForUser(gomock.Any(), student.ID, []uint{9}).Return([]model.OfflinePackage{p}, nil)
	env.packages.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)

	resp, err := env.api.SyncOfflineLicenses(asCaller(student), &protobuf.SyncOfflineLicensesRequest{PackageIds: []uint32{9}})
	if err != nil {
		t.Fatalf("SyncOfflineLicenses = %v", err)
	}
	if len(resp.Licenses) != 1 || !resp.Licenses[0].Revoked {
		t.Errorf("licences = %v, want 9 revoked", resp.Licenses)
	}
	// The package was downloaded already, so there is nothing on the server to delete.
	if len(env.packager.deleted) != 0 {
		t.Errorf("deleted %v", env.packager.deleted)
	}
}
