package runner_manager

import (
	"context"
	"testing"

	"go.uber.org/mock/gomock"

	"github.com/tum-dev/gocast/runner/pkg/ptr"
	"github.com/tum-dev/gocast/runner/protobuf"

	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/mock_dao"
	"github.com/TUM-Dev/gocast/model"
)

func TestOfflinePackageReadyMarksThePackageReady(t *testing.T) {
	ctrl := gomock.NewController(t)
	packages := mock_dao.NewMockOfflinePackageDao(ctrl)
	packages.EXPECT().Get(gomock.Any(), uint(5)).Return(model.OfflinePackage{ID: 5, State: model.OfflinePreparing}, nil)
	packages.EXPECT().Update(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, p *model.OfflinePackage) error {
		if p.State != model.OfflineReady || p.SegmentCount != 3 || p.SizeBytes != 1000 {
			t.Errorf("updated package = %+v", p)
		}
		return nil
	})
	m := New(dao.DaoWrapper{OfflinePackageDao: packages})

	_, err := m.Notify(context.Background(), &protobuf.Notification{Data: &protobuf.Notification_OfflinePackageReady{
		OfflinePackageReady: &protobuf.OfflinePackageReadyNotification{
			PackageId: ptr.Take(uint64(5)), SegmentCount: ptr.Take(uint32(3)), SizeBytes: ptr.Take(uint64(1000)),
		},
	}})
	if err != nil {
		t.Fatalf("Notify = %v", err)
	}
}

// A notification is retried until it is acknowledged, so a late one can arrive after
// the device downloaded the package or the viewer deleted it.
func TestLateOfflineNotificationsChangeNothing(t *testing.T) {
	for _, state := range []model.OfflinePackageState{model.OfflineDownloaded, model.OfflineRevoked, model.OfflineFailed} {
		ctrl := gomock.NewController(t)
		packages := mock_dao.NewMockOfflinePackageDao(ctrl)
		packages.EXPECT().Get(gomock.Any(), uint(5)).Return(model.OfflinePackage{ID: 5, State: state}, nil).Times(2)
		// No Update expected.
		m := New(dao.DaoWrapper{OfflinePackageDao: packages})

		ready := &protobuf.Notification{Data: &protobuf.Notification_OfflinePackageReady{
			OfflinePackageReady: &protobuf.OfflinePackageReadyNotification{PackageId: ptr.Take(uint64(5))},
		}}
		failed := &protobuf.Notification{Data: &protobuf.Notification_OfflinePackageFailed{
			OfflinePackageFailed: &protobuf.OfflinePackageFailedNotification{PackageId: ptr.Take(uint64(5)), Reason: ptr.Take("x")},
		}}
		for _, n := range []*protobuf.Notification{ready, failed} {
			if _, err := m.Notify(context.Background(), n); err != nil {
				t.Errorf("%s: Notify = %v", state, err)
			}
		}
	}
}
