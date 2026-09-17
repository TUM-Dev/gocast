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
	"gorm.io/gorm"

	protobuf "github.com/TUM-Dev/gocast/apiv2/protobuf/server"
	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/mock_dao"
	"github.com/TUM-Dev/gocast/model"
)

func TestListLectureHallsAdmin(t *testing.T) {
	ctrl := gomock.NewController(t)
	lhMock := mock_dao.NewMockLectureHallsDao(ctrl)
	lhMock.EXPECT().GetAllLectureHalls().Return([]model.LectureHall{
		{Model: gorm.Model{ID: 1}, Name: "FMI_HS1", StreamProtocol: model.RTSP, CamIP: "rtsp://0.0.0.0/cam"},
	}).Times(1)

	api := &API{dao: dao.DaoWrapper{LectureHallsDao: lhMock}, log: slog.Default()}

	resp, err := api.ListLectureHallsAdmin(context.Background(), &emptypb.Empty{})
	if err != nil {
		t.Fatalf("ListLectureHallsAdmin: %v", err)
	}
	if len(resp.LectureHalls) != 1 || resp.LectureHalls[0].Name != "FMI_HS1" {
		t.Fatalf("got %+v, want one hall named FMI_HS1", resp.LectureHalls)
	}
	if resp.LectureHalls[0].Id != 1 {
		t.Errorf("id = %d, want 1", resp.LectureHalls[0].Id)
	}
}

func TestCreateLectureHallAdmin(t *testing.T) {
	t.Run("creates a hall with a name", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		lhMock := mock_dao.NewMockLectureHallsDao(ctrl)
		lhMock.EXPECT().CreateLectureHall(gomock.Any()).DoAndReturn(func(lh *model.LectureHall) error {
			lh.ID = 9
			return nil
		}).Times(1)

		api := &API{dao: dao.DaoWrapper{LectureHallsDao: lhMock}, log: slog.Default()}

		resp, err := api.CreateLectureHallAdmin(context.Background(), &protobuf.CreateLectureHallAdminRequest{
			Name: "  FMI_HS1  ", StreamProtocol: 1, CamIp: " rtsp://0.0.0.0/cam ",
		})
		if err != nil {
			t.Fatalf("CreateLectureHallAdmin: %v", err)
		}
		if resp.Id != 9 || resp.Name != "FMI_HS1" {
			t.Errorf("got %+v", resp)
		}
		// The caller's stray whitespace should not survive into the stored address.
		if resp.CamIp != "rtsp://0.0.0.0/cam" {
			t.Errorf("camIp = %q, want it trimmed", resp.CamIp)
		}
	})

	t.Run("rejects a blank name", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		lhMock := mock_dao.NewMockLectureHallsDao(ctrl)
		lhMock.EXPECT().CreateLectureHall(gomock.Any()).Times(0)

		api := &API{dao: dao.DaoWrapper{LectureHallsDao: lhMock}, log: slog.Default()}

		_, err := api.CreateLectureHallAdmin(context.Background(), &protobuf.CreateLectureHallAdminRequest{
			Name: "   ",
		})
		if err == nil {
			t.Fatal("a blank name was accepted")
		}
	})
}

func TestUpdateLectureHallAdmin(t *testing.T) {
	t.Run("updates an existing hall", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		lhMock := mock_dao.NewMockLectureHallsDao(ctrl)
		lhMock.EXPECT().GetLectureHallByID(uint(3)).
			Return(model.LectureHall{Model: gorm.Model{ID: 3}, Name: "Old Name"}, nil).Times(1)
		lhMock.EXPECT().SaveLectureHall(gomock.Any()).Return(nil).Times(1)

		api := &API{dao: dao.DaoWrapper{LectureHallsDao: lhMock}, log: slog.Default()}

		resp, err := api.UpdateLectureHallAdmin(context.Background(), &protobuf.UpdateLectureHallAdminRequest{
			Id: 3, Name: "New Name", StreamProtocol: 2,
		})
		if err != nil {
			t.Fatalf("UpdateLectureHallAdmin: %v", err)
		}
		if resp.Name != "New Name" || resp.StreamProtocol != 2 {
			t.Errorf("got %+v", resp)
		}
	})

	t.Run("404s an id nothing was seeded for", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		lhMock := mock_dao.NewMockLectureHallsDao(ctrl)
		lhMock.EXPECT().GetLectureHallByID(uint(99)).Return(model.LectureHall{}, gorm.ErrRecordNotFound).Times(1)
		lhMock.EXPECT().SaveLectureHall(gomock.Any()).Times(0)

		api := &API{dao: dao.DaoWrapper{LectureHallsDao: lhMock}, log: slog.Default()}

		_, err := api.UpdateLectureHallAdmin(context.Background(), &protobuf.UpdateLectureHallAdminRequest{
			Id: 99, Name: "New Name",
		})
		if err == nil {
			t.Fatal("a missing hall was not reported as an error")
		}
	})

	t.Run("rejects a blank name", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		lhMock := mock_dao.NewMockLectureHallsDao(ctrl)
		lhMock.EXPECT().GetLectureHallByID(gomock.Any()).Times(0)
		lhMock.EXPECT().SaveLectureHall(gomock.Any()).Times(0)

		api := &API{dao: dao.DaoWrapper{LectureHallsDao: lhMock}, log: slog.Default()}

		_, err := api.UpdateLectureHallAdmin(context.Background(), &protobuf.UpdateLectureHallAdminRequest{
			Id: 3, Name: "   ",
		})
		if err == nil {
			t.Fatal("a blank name was accepted")
		}
	})
}

func TestDeleteLectureHallAdmin(t *testing.T) {
	t.Run("deletes the hall it was given", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		lhMock := mock_dao.NewMockLectureHallsDao(ctrl)
		lhMock.EXPECT().DeleteLectureHall(uint(3)).Return(nil).Times(1)

		api := &API{dao: dao.DaoWrapper{LectureHallsDao: lhMock}, log: slog.Default()}

		if _, err := api.DeleteLectureHallAdmin(context.Background(), &protobuf.DeleteLectureHallAdminRequest{Id: 3}); err != nil {
			t.Fatalf("DeleteLectureHallAdmin: %v", err)
		}
	})

	t.Run("reports a failed delete", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		lhMock := mock_dao.NewMockLectureHallsDao(ctrl)
		lhMock.EXPECT().DeleteLectureHall(gomock.Any()).Return(errors.New("database is on fire")).Times(1)

		api := &API{dao: dao.DaoWrapper{LectureHallsDao: lhMock}, log: slog.Default()}

		_, err := api.DeleteLectureHallAdmin(context.Background(), &protobuf.DeleteLectureHallAdminRequest{Id: 3})
		if err == nil {
			t.Fatal("a failed delete was reported as success")
		}
	})
}

func TestRefreshLectureHallPresetsAdmin(t *testing.T) {
	t.Run("fetches presets from the camera and stores them", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		lhMock := mock_dao.NewMockLectureHallsDao(ctrl)
		hall := model.LectureHall{Model: gorm.Model{ID: 1}, Name: "FMI_HS1", CameraIP: "10.0.0.5"}
		lhMock.EXPECT().GetLectureHallByID(uint(1)).Return(hall, nil).Times(1)
		lhMock.EXPECT().SaveLectureHallFullAssoc(gomock.Any()).Times(1)
		lhMock.EXPECT().GetLectureHallByID(uint(1)).Return(model.LectureHall{
			Model: gorm.Model{ID: 1}, Name: "FMI_HS1", CameraIP: "10.0.0.5",
			CameraPresets: []model.CameraPreset{{Name: "Front", PresetID: 1, LectureHallID: 1}},
		}, nil).Times(1)

		cam := &fakeCam{presets: []model.CameraPreset{{Name: "Front", PresetID: 1}}}
		api := &API{
			dao:  dao.DaoWrapper{LectureHallsDao: lhMock},
			log:  slog.Default(),
			cams: &fakeCamService{cam: cam},
		}

		resp, err := api.RefreshLectureHallPresetsAdmin(context.Background(), &protobuf.RefreshLectureHallPresetsAdminRequest{Id: 1})
		if err != nil {
			t.Fatalf("RefreshLectureHallPresetsAdmin: %v", err)
		}
		if len(resp.CameraPresets) != 1 || resp.CameraPresets[0].Name != "Front" {
			t.Errorf("got %+v, want one preset named Front", resp.CameraPresets)
		}
	})

	t.Run("reports a hall without a camera rather than saving anything", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		lhMock := mock_dao.NewMockLectureHallsDao(ctrl)
		lhMock.EXPECT().GetLectureHallByID(uint(1)).Return(model.LectureHall{Model: gorm.Model{ID: 1}}, nil).Times(1)
		lhMock.EXPECT().SaveLectureHallFullAssoc(gomock.Any()).Times(0)

		api := &API{dao: dao.DaoWrapper{LectureHallsDao: lhMock}, log: slog.Default(), cams: &fakeCamService{cam: &fakeCam{}}}

		_, err := api.RefreshLectureHallPresetsAdmin(context.Background(), &protobuf.RefreshLectureHallPresetsAdminRequest{Id: 1})
		if status.Code(err) != codes.InvalidArgument {
			t.Fatalf("code = %v, want InvalidArgument", status.Code(err))
		}
	})
}

func TestSetDefaultCameraPresetAdmin(t *testing.T) {
	ctrl := gomock.NewController(t)
	lhMock := mock_dao.NewMockLectureHallsDao(ctrl)
	preset := model.CameraPreset{Name: "Front", PresetID: 1, LectureHallID: 1}
	lhMock.EXPECT().FindPreset("1", "1").Return(preset, nil).Times(1)
	lhMock.EXPECT().UnsetDefaults("1").Return(nil).Times(1)
	lhMock.EXPECT().SavePreset(gomock.Any()).DoAndReturn(func(p model.CameraPreset) error {
		if !p.IsDefault {
			t.Error("saved preset is not marked default")
		}
		return nil
	}).Times(1)

	api := &API{dao: dao.DaoWrapper{LectureHallsDao: lhMock}, log: slog.Default()}

	_, err := api.SetDefaultCameraPresetAdmin(context.Background(), &protobuf.SetDefaultCameraPresetAdminRequest{
		LectureHallId: 1, PresetId: 1,
	})
	if err != nil {
		t.Fatalf("SetDefaultCameraPresetAdmin: %v", err)
	}
}

func TestTakeCameraPresetSnapshotAdmin(t *testing.T) {
	restore := camSwitchDelay
	camSwitchDelay = time.Millisecond
	defer func() { camSwitchDelay = restore }()

	t.Run("waits for the camera, then photographs and saves the preset", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		lhMock := mock_dao.NewMockLectureHallsDao(ctrl)
		preset := model.CameraPreset{Name: "Front", PresetID: 1, LectureHallID: 1}
		lhMock.EXPECT().FindPreset("1", "1").Return(preset, nil).Times(1)
		lhMock.EXPECT().GetLectureHallByID(uint(1)).Return(model.LectureHall{
			Model: gorm.Model{ID: 1}, CameraIP: "10.0.0.5",
		}, nil).Times(1)
		lhMock.EXPECT().SavePreset(gomock.Any()).DoAndReturn(func(p model.CameraPreset) error {
			if p.Image != "snapshot.jpg" {
				t.Errorf("saved image = %q, want snapshot.jpg", p.Image)
			}
			return nil
		}).Times(1)

		cam := &fakeCam{}
		api := &API{
			dao:            dao.DaoWrapper{LectureHallsDao: lhMock},
			log:            slog.Default(),
			cams:           &fakeCamService{cam: cam},
			presetImageDir: "/srv/static",
		}

		resp, err := api.TakeCameraPresetSnapshotAdmin(context.Background(), &protobuf.TakeCameraPresetSnapshotAdminRequest{
			LectureHallId: 1, PresetId: 1,
		})
		if err != nil {
			t.Fatalf("TakeCameraPresetSnapshotAdmin: %v", err)
		}
		if resp.Image != "snapshot.jpg" {
			t.Errorf("image = %q, want snapshot.jpg", resp.Image)
		}
		if len(cam.setPresetIDs) != 1 || cam.setPresetIDs[0] != 1 {
			t.Errorf("SetPreset calls = %v, want [1]", cam.setPresetIDs)
		}
		if len(cam.snapshotDirs) != 1 || cam.snapshotDirs[0] != "/srv/static" {
			t.Errorf("TakeSnapshot dirs = %v, want [/srv/static]", cam.snapshotDirs)
		}
	})

	t.Run("refuses without a configured snapshot directory", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		lhMock := mock_dao.NewMockLectureHallsDao(ctrl)
		preset := model.CameraPreset{Name: "Front", PresetID: 1, LectureHallID: 1}
		lhMock.EXPECT().FindPreset("1", "1").Return(preset, nil).Times(1)
		lhMock.EXPECT().GetLectureHallByID(uint(1)).Return(model.LectureHall{
			Model: gorm.Model{ID: 1}, CameraIP: "10.0.0.5",
		}, nil).Times(1)
		lhMock.EXPECT().SavePreset(gomock.Any()).Times(0)

		api := &API{dao: dao.DaoWrapper{LectureHallsDao: lhMock}, log: slog.Default(), cams: &fakeCamService{cam: &fakeCam{}}}

		_, err := api.TakeCameraPresetSnapshotAdmin(context.Background(), &protobuf.TakeCameraPresetSnapshotAdminRequest{
			LectureHallId: 1, PresetId: 1,
		})
		if err == nil {
			t.Fatal("took a snapshot with nowhere configured to put it")
		}
	})

	t.Run("cancellation during the switch delay is reported rather than photographed", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		lhMock := mock_dao.NewMockLectureHallsDao(ctrl)
		preset := model.CameraPreset{Name: "Front", PresetID: 1, LectureHallID: 1}
		lhMock.EXPECT().FindPreset("1", "1").Return(preset, nil).Times(1)
		lhMock.EXPECT().GetLectureHallByID(uint(1)).Return(model.LectureHall{
			Model: gorm.Model{ID: 1}, CameraIP: "10.0.0.5",
		}, nil).Times(1)
		lhMock.EXPECT().SavePreset(gomock.Any()).Times(0)

		camSwitchDelay = time.Hour
		defer func() { camSwitchDelay = time.Millisecond }()

		cam := &fakeCam{}
		api := &API{dao: dao.DaoWrapper{LectureHallsDao: lhMock}, log: slog.Default(), cams: &fakeCamService{cam: cam}, presetImageDir: "/srv/static"}

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		_, err := api.TakeCameraPresetSnapshotAdmin(ctx, &protobuf.TakeCameraPresetSnapshotAdminRequest{
			LectureHallId: 1, PresetId: 1,
		})
		if status.Code(err) != codes.Canceled {
			t.Fatalf("code = %v, want Canceled", status.Code(err))
		}
		if len(cam.snapshotDirs) != 0 {
			t.Error("took a snapshot despite the cancelled context")
		}
	})
}
