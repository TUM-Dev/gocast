package apiv2

import (
	"context"
	"database/sql"
	"log/slog"
	"testing"

	"go.uber.org/mock/gomock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"

	protobuf "github.com/TUM-Dev/gocast/apiv2/protobuf/server"
	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/mock_dao"
	"github.com/TUM-Dev/gocast/model"
)

// unitsLecture is stream 7 of course 5, with two units.
func unitsLecture() model.Stream {
	return model.Stream{
		Model: gorm.Model{ID: 7}, CourseID: 5, Name: "Hopfen",
		PlaylistUrl: "https://vod.example/7.m3u8",
		Duration:    sql.NullInt32{Int32: 5400, Valid: true},
		Units: []model.StreamUnit{
			{Model: gorm.Model{ID: 1}, UnitName: "Intro", UnitDescription: "*hi*", UnitStart: 0, UnitEnd: 60000, StreamID: 7},
			{Model: gorm.Model{ID: 2}, UnitName: "Malz", UnitStart: 60000, UnitEnd: 120000, StreamID: 7},
		},
	}
}

func unitsAPI(t *testing.T, stream model.Stream) (*API, *mock_dao.MockStreamsDao) {
	t.Helper()
	noSigning(t)
	streams := mock_dao.NewMockStreamsDao(gomock.NewController(t))
	streams.EXPECT().GetStreamByID(gomock.Any(), "7").Return(stream, nil).AnyTimes()
	return &API{dao: dao.DaoWrapper{StreamsDao: streams}, log: slog.Default()}, streams
}

func wantUnitsCode(t *testing.T, err error, want codes.Code) {
	t.Helper()
	if status.Code(err) != want {
		t.Fatalf("code = %v (%v), want %v", status.Code(err), err, want)
	}
}

func TestLectureUnitsArePolicedByCourse(t *testing.T) {
	for _, name := range []string{"listLectureUnits", "createLectureUnit", "deleteLectureUnit"} {
		if p := methodPolicies[method(&protobuf.CourseService_ServiceDesc, name)]; !p.courseScoped {
			t.Errorf("%s is not course-scoped: %+v", name, p)
		}
	}
}

func TestListLectureUnits(t *testing.T) {
	t.Run("answers the units in order with the signed recording", func(t *testing.T) {
		api, _ := unitsAPI(t, unitsLecture())

		resp, err := api.ListLectureUnits(context.Background(), &protobuf.ListLectureUnitsRequest{CourseId: 5, StreamId: 7})
		if err != nil {
			t.Fatalf("ListLectureUnits: %v", err)
		}
		if resp.LectureName != "Hopfen" || resp.PlaylistUrl != "https://vod.example/7.m3u8?jwt=signed" || resp.DurationSeconds != 5400 {
			t.Errorf("header = %q / %q / %d", resp.LectureName, resp.PlaylistUrl, resp.DurationSeconds)
		}
		if len(resp.Units) != 2 || resp.Units[0].Title != "Intro" || resp.Units[0].Description != "*hi*" ||
			resp.Units[1].FromMs != 60000 || resp.Units[1].ToMs != 120000 || resp.Units[1].Id != 2 {
			t.Errorf("units = %v", resp.Units)
		}
	})

	t.Run("answers an empty list, not null, for a lecture with no units and no recording", func(t *testing.T) {
		bare := model.Stream{Model: gorm.Model{ID: 7}, CourseID: 5, Name: "Leer"}
		api, _ := unitsAPI(t, bare)

		resp, err := api.ListLectureUnits(context.Background(), &protobuf.ListLectureUnitsRequest{CourseId: 5, StreamId: 7})
		if err != nil {
			t.Fatalf("ListLectureUnits: %v", err)
		}
		if resp.Units == nil || len(resp.Units) != 0 || resp.PlaylistUrl != "" || resp.DurationSeconds != 0 {
			t.Errorf("resp = %v", resp)
		}
	})

	t.Run("falls back to the presentation, then the camera", func(t *testing.T) {
		pres := unitsLecture()
		pres.PlaylistUrl, pres.PlaylistUrlPRES, pres.PlaylistUrlCAM = "", "https://vod.example/p.m3u8", "https://vod.example/c.m3u8"
		api, _ := unitsAPI(t, pres)
		resp, err := api.ListLectureUnits(context.Background(), &protobuf.ListLectureUnitsRequest{CourseId: 5, StreamId: 7})
		if err != nil || resp.PlaylistUrl != "https://vod.example/p.m3u8?jwt=signed" {
			t.Fatalf("pres: %v, %v", resp, err)
		}

		cam := pres
		cam.PlaylistUrlPRES = ""
		api, _ = unitsAPI(t, cam)
		resp, err = api.ListLectureUnits(context.Background(), &protobuf.ListLectureUnitsRequest{CourseId: 5, StreamId: 7})
		if err != nil || resp.PlaylistUrl != "https://vod.example/c.m3u8" {
			t.Fatalf("cam: %v, %v", resp, err)
		}
	})

	t.Run("refuses a lecture of another course as a missing one", func(t *testing.T) {
		api, _ := unitsAPI(t, unitsLecture())

		_, err := api.ListLectureUnits(context.Background(), &protobuf.ListLectureUnitsRequest{CourseId: 6, StreamId: 7})
		wantUnitsCode(t, err, codes.NotFound)
	})
}

func TestCreateLectureUnit(t *testing.T) {
	req := func() *protobuf.CreateLectureUnitRequest {
		return &protobuf.CreateLectureUnitRequest{
			CourseId: 5, StreamId: 7, Title: "  Hopfen  ", Description: "text", FromMs: 1000, ToMs: 9000,
		}
	}

	t.Run("stores the unit on the lecture and answers it", func(t *testing.T) {
		api, streams := unitsAPI(t, unitsLecture())
		streams.EXPECT().CreateUnit(gomock.Any()).DoAndReturn(func(u *model.StreamUnit) error {
			if u.StreamID != 7 || u.UnitName != "Hopfen" || u.UnitDescription != "text" || u.UnitStart != 1000 || u.UnitEnd != 9000 {
				t.Errorf("stored %+v", u)
			}
			u.ID = 42
			return nil
		})

		got, err := api.CreateLectureUnit(context.Background(), req())
		if err != nil {
			t.Fatalf("CreateLectureUnit: %v", err)
		}
		if got.Id != 42 || got.Title != "Hopfen" || got.FromMs != 1000 || got.ToMs != 9000 {
			t.Errorf("unit = %v", got)
		}
	})

	t.Run("refuses a blank title and an empty or backwards range, storing nothing", func(t *testing.T) {
		api, _ := unitsAPI(t, unitsLecture()) // CreateUnit is unexpected: a call fails the test.

		blank := req()
		blank.Title = " \t"
		_, err := api.CreateLectureUnit(context.Background(), blank)
		wantUnitsCode(t, err, codes.InvalidArgument)

		empty := req()
		empty.ToMs = empty.FromMs
		_, err = api.CreateLectureUnit(context.Background(), empty)
		wantUnitsCode(t, err, codes.InvalidArgument)

		backwards := req()
		backwards.FromMs, backwards.ToMs = 9000, 1000
		_, err = api.CreateLectureUnit(context.Background(), backwards)
		wantUnitsCode(t, err, codes.InvalidArgument)
	})

	t.Run("refuses a lecture of another course, storing nothing", func(t *testing.T) {
		api, _ := unitsAPI(t, unitsLecture())

		foreign := req()
		foreign.CourseId = 6
		_, err := api.CreateLectureUnit(context.Background(), foreign)
		wantUnitsCode(t, err, codes.NotFound)
	})
}

func TestDeleteLectureUnit(t *testing.T) {
	t.Run("removes the lecture's unit", func(t *testing.T) {
		api, streams := unitsAPI(t, unitsLecture())
		streams.EXPECT().GetUnitByID("2").Return(unitsLecture().Units[1], nil)
		streams.EXPECT().DeleteUnit(uint(2))

		if _, err := api.DeleteLectureUnit(context.Background(), &protobuf.DeleteLectureUnitRequest{CourseId: 5, StreamId: 7, UnitId: 2}); err != nil {
			t.Fatalf("DeleteLectureUnit: %v", err)
		}
	})

	t.Run("refuses a unit of another lecture as a missing one, removing nothing", func(t *testing.T) {
		api, streams := unitsAPI(t, unitsLecture())
		streams.EXPECT().GetUnitByID("9").Return(model.StreamUnit{Model: gorm.Model{ID: 9}, StreamID: 8}, nil)

		_, err := api.DeleteLectureUnit(context.Background(), &protobuf.DeleteLectureUnitRequest{CourseId: 5, StreamId: 7, UnitId: 9})
		wantUnitsCode(t, err, codes.NotFound)
	})

	t.Run("answers a unit that does not exist as missing", func(t *testing.T) {
		api, streams := unitsAPI(t, unitsLecture())
		streams.EXPECT().GetUnitByID("9").Return(model.StreamUnit{}, gorm.ErrRecordNotFound)

		_, err := api.DeleteLectureUnit(context.Background(), &protobuf.DeleteLectureUnitRequest{CourseId: 5, StreamId: 7, UnitId: 9})
		wantUnitsCode(t, err, codes.NotFound)
	})

	t.Run("refuses a lecture of another course, removing nothing", func(t *testing.T) {
		api, _ := unitsAPI(t, unitsLecture())

		_, err := api.DeleteLectureUnit(context.Background(), &protobuf.DeleteLectureUnitRequest{CourseId: 6, StreamId: 7, UnitId: 2})
		wantUnitsCode(t, err, codes.NotFound)
	})
}
