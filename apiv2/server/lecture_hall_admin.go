package apiv2

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"google.golang.org/protobuf/types/known/emptypb"

	e "github.com/TUM-Dev/gocast/apiv2/errors"
	protobuf "github.com/TUM-Dev/gocast/apiv2/protobuf/server"
	"github.com/TUM-Dev/gocast/model"
)

// Every RPC here is gated on PermAdministerServer by its policy in services.go.
//
// Camera preset management -- viewing the presets already fetched from a hall's
// camera, refreshing them, marking a default and taking a new snapshot -- is
// deliberately not part of this surface. It needs the CamService and the preset
// image directory, which only the v1 handler in api/lecture_halls.go has wired up,
// and folding that into the v2 API is a bigger change than this page's migration.

// ListLectureHallsAdmin lists every lecture hall with its stream sources.
func (a *API) ListLectureHallsAdmin(ctx context.Context, req *emptypb.Empty) (*protobuf.ListLectureHallsAdminResponse, error) {
	lectureHalls := a.dao.LectureHallsDao.GetAllLectureHalls()

	out := make([]*protobuf.LectureHallAdmin, 0, len(lectureHalls))
	for _, lh := range lectureHalls {
		out = append(out, lectureHallAdminMessage(lh))
	}

	return &protobuf.ListLectureHallsAdminResponse{LectureHalls: out}, nil
}

// CreateLectureHallAdmin creates a lecture hall the scheduler can assign lectures to.
func (a *API) CreateLectureHallAdmin(ctx context.Context, req *protobuf.CreateLectureHallAdminRequest) (*protobuf.LectureHallAdmin, error) {
	name := strings.TrimSpace(req.GetName())
	if name == "" {
		return nil, e.WithStatus(http.StatusBadRequest, errors.New("name can not be empty"))
	}

	lectureHall := model.LectureHall{
		Name:           name,
		StreamProtocol: model.StreamProtocol(req.GetStreamProtocol()),
		CombIP:         strings.TrimSpace(req.GetCombIp()),
		PresIP:         strings.TrimSpace(req.GetPresIp()),
		CamIP:          strings.TrimSpace(req.GetCamIp()),
		CameraIP:       strings.TrimSpace(req.GetCameraIp()),
		PwrCtrlIp:      strings.TrimSpace(req.GetPwrCtrlIp()),
	}

	if err := a.dao.LectureHallsDao.CreateLectureHall(&lectureHall); err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}

	return lectureHallAdminMessage(lectureHall), nil
}

// UpdateLectureHallAdmin replaces a lecture hall's name, stream protocol and sources.
func (a *API) UpdateLectureHallAdmin(ctx context.Context, req *protobuf.UpdateLectureHallAdminRequest) (*protobuf.LectureHallAdmin, error) {
	name := strings.TrimSpace(req.GetName())
	if name == "" {
		return nil, e.WithStatus(http.StatusBadRequest, errors.New("name can not be empty"))
	}

	lectureHall, err := a.dao.LectureHallsDao.GetLectureHallByID(uint(req.GetId()))
	if err != nil {
		return nil, e.FromGorm(err, "can not find lecture hall")
	}

	lectureHall.Name = name
	lectureHall.StreamProtocol = model.StreamProtocol(req.GetStreamProtocol())
	lectureHall.CombIP = strings.TrimSpace(req.GetCombIp())
	lectureHall.PresIP = strings.TrimSpace(req.GetPresIp())
	lectureHall.CamIP = strings.TrimSpace(req.GetCamIp())
	lectureHall.CameraIP = strings.TrimSpace(req.GetCameraIp())
	lectureHall.PwrCtrlIp = strings.TrimSpace(req.GetPwrCtrlIp())

	if err := a.dao.LectureHallsDao.SaveLectureHall(lectureHall); err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}

	return lectureHallAdminMessage(lectureHall), nil
}

// DeleteLectureHallAdmin deletes a lecture hall. Its streams keep playing but lose
// the hall association, matching the v1 handler's DeleteLectureHall dao call.
func (a *API) DeleteLectureHallAdmin(ctx context.Context, req *protobuf.DeleteLectureHallAdminRequest) (*emptypb.Empty, error) {
	if err := a.dao.LectureHallsDao.DeleteLectureHall(uint(req.GetId())); err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}
	return &emptypb.Empty{}, nil
}

func lectureHallAdminMessage(lh model.LectureHall) *protobuf.LectureHallAdmin {
	return &protobuf.LectureHallAdmin{
		Id:             uint32(lh.ID),
		Name:           lh.Name,
		StreamProtocol: uint32(lh.StreamProtocol),
		CombIp:         lh.CombIP,
		PresIp:         lh.PresIP,
		CamIp:          lh.CamIP,
		CameraIp:       lh.CameraIP,
		PwrCtrlIp:      lh.PwrCtrlIp,
	}
}
