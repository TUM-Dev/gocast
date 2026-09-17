package apiv2

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"go.uber.org/mock/gomock"
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
