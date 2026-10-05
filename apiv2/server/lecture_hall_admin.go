package apiv2

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	e "github.com/TUM-Dev/gocast/apiv2/errors"
	protobuf "github.com/TUM-Dev/gocast/apiv2/protobuf/server"
	"github.com/TUM-Dev/gocast/model"
)

// Every RPC here is gated on PermAdministerServer by its policy in services.go.

// camSwitchDelay is how long a camera takes to physically move to a preset before a
// snapshot of it means anything, rather than a photo of wherever it was pointed
// before. Mirrors api.camSwitchDelay, the v1 handler this replaces.
var camSwitchDelay = 5 * time.Second

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

// RefreshLectureHallPresetsAdmin fetches the presets configured on a hall's camera
// and replaces the stored list with them, matching v1's fetchLHPresets.
func (a *API) RefreshLectureHallPresetsAdmin(ctx context.Context, req *protobuf.RefreshLectureHallPresetsAdminRequest) (*protobuf.LectureHallAdmin, error) {
	lectureHall, err := a.dao.LectureHallsDao.GetLectureHallByID(uint(req.GetId()))
	if err != nil {
		return nil, e.FromGorm(err, "can not find lecture hall")
	}

	cam, err := a.cameraFor(lectureHall)
	if err != nil {
		return nil, err
	}

	presets, err := cam.GetPresets()
	if err != nil {
		return nil, e.WithStatus(http.StatusServiceUnavailable, err)
	}

	lectureHall.CameraPresets = presets
	a.dao.LectureHallsDao.SaveLectureHallFullAssoc(lectureHall)

	// SaveLectureHallFullAssoc does not report what it wrote back (it returns
	// nothing), and it is what fills in each preset's LectureHallID -- so the
	// message is built from a fresh read rather than the in-memory presets.
	lectureHall, err = a.dao.LectureHallsDao.GetLectureHallByID(lectureHall.ID)
	if err != nil {
		return nil, e.FromGorm(err, "can not find lecture hall")
	}

	return lectureHallAdminMessage(lectureHall), nil
}

// SetDefaultCameraPresetAdmin marks one preset default for a hall and unsets the
// others, matching v1's updateLectureHallsDefaultPreset.
func (a *API) SetDefaultCameraPresetAdmin(ctx context.Context, req *protobuf.SetDefaultCameraPresetAdminRequest) (*emptypb.Empty, error) {
	lectureHallID := fmt.Sprintf("%d", req.GetLectureHallId())

	preset, err := a.dao.LectureHallsDao.FindPreset(lectureHallID, fmt.Sprintf("%d", req.GetPresetId()))
	if err != nil {
		return nil, e.FromGorm(err, "can not find preset")
	}
	preset.IsDefault = true

	if err := a.dao.LectureHallsDao.UnsetDefaults(lectureHallID); err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}

	if err := a.dao.LectureHallsDao.SavePreset(preset); err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}

	return &emptypb.Empty{}, nil
}

// TakeCameraPresetSnapshotAdmin moves the camera to the preset, waits for it to
// arrive, and photographs it, matching v1's takeSnapshot.
func (a *API) TakeCameraPresetSnapshotAdmin(ctx context.Context, req *protobuf.TakeCameraPresetSnapshotAdminRequest) (*protobuf.CameraPresetAdmin, error) {
	preset, err := a.dao.LectureHallsDao.FindPreset(
		fmt.Sprintf("%d", req.GetLectureHallId()), fmt.Sprintf("%d", req.GetPresetId()),
	)
	if err != nil {
		return nil, e.FromGorm(err, "can not find preset")
	}

	lectureHall, err := a.dao.LectureHallsDao.GetLectureHallByID(preset.LectureHallID)
	if err != nil {
		return nil, e.FromGorm(err, "can not find lecture hall")
	}

	cam, err := a.cameraFor(lectureHall)
	if err != nil {
		return nil, err
	}

	if err := cam.SetPreset(preset.PresetID); err != nil {
		return nil, e.WithStatus(http.StatusServiceUnavailable, err)
	}

	// The camera needs time to physically move; a snapshot taken now would show
	// wherever it was pointed before this call.
	select {
	case <-ctx.Done():
		return nil, status.FromContextError(ctx.Err()).Err()
	case <-time.After(camSwitchDelay):
	}

	dir, err := a.snapshotDir()
	if err != nil {
		return nil, err
	}

	image, err := cam.TakeSnapshot(dir)
	if err != nil {
		return nil, e.WithStatus(http.StatusServiceUnavailable, err)
	}
	preset.Image = image

	if err := a.dao.LectureHallsDao.SavePreset(preset); err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}

	return cameraPresetAdminMessage(preset), nil
}

func lectureHallAdminMessage(lh model.LectureHall) *protobuf.LectureHallAdmin {
	presets := make([]*protobuf.CameraPresetAdmin, 0, len(lh.CameraPresets))
	for _, preset := range lh.CameraPresets {
		presets = append(presets, cameraPresetAdminMessage(preset))
	}

	return &protobuf.LectureHallAdmin{
		Id:             uint32(lh.ID),
		Name:           lh.Name,
		StreamProtocol: uint32(lh.StreamProtocol),
		CombIp:         lh.CombIP,
		PresIp:         lh.PresIP,
		CamIp:          lh.CamIP,
		CameraIp:       lh.CameraIP,
		PwrCtrlIp:      lh.PwrCtrlIp,
		CameraPresets:  presets,
	}
}

func cameraPresetAdminMessage(preset model.CameraPreset) *protobuf.CameraPresetAdmin {
	return &protobuf.CameraPresetAdmin{
		LectureHallId: uint32(preset.LectureHallID),
		PresetId:      uint32(preset.PresetID),
		Name:          preset.Name,
		Image:         preset.Image,
		IsDefault:     preset.IsDefault,
	}
}
