package apiv2

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"time"

	uuid "github.com/satori/go.uuid"
	"google.golang.org/protobuf/types/known/emptypb"
	"gorm.io/gorm"

	e "github.com/TUM-Dev/gocast/apiv2/errors"
	protobuf "github.com/TUM-Dev/gocast/apiv2/protobuf/server"
	"github.com/TUM-Dev/gocast/model"
)

func (a *API) GetIntegration(ctx context.Context, _ *emptypb.Empty) (*protobuf.GetIntegrationResponse, error) {
	integration, err := a.getCurrentIntegration(ctx)
	if err != nil {
		return nil, err
	}
	return &protobuf.GetIntegrationResponse{
		Id:        uint32(integration.ID),
		Name:      integration.Name,
		ReturnUrl: integration.ReturnURL,
	}, nil
}

func (a *API) RedeemIntegrationAuthorization(ctx context.Context, req *protobuf.RedeemIntegrationAuthorizationRequest) (*protobuf.RedeemIntegrationAuthorizationResponse, error) {
	code, err := uuid.FromString(req.GetCode())
	if err != nil || code.String() != req.GetCode() || code.Version() != uuid.V4 || code.Variant() != uuid.VariantRFC4122 {
		return nil, e.WithStatus(http.StatusBadRequest, errors.New("code must be a canonical version 4 UUID"))
	}
	state, ok := validAuthorizationState(req.GetState())
	if !ok {
		return nil, e.WithStatus(http.StatusBadRequest, errors.New("state must encode 32 bytes as unpadded base64url"))
	}
	integration, err := a.getCurrentIntegration(ctx)
	if err != nil {
		return nil, err
	}
	codeHash, stateHash := sha256.Sum256(code[:]), sha256.Sum256(state)
	grantID, courseID, err := a.dao.IntegrationGrantDao.RedeemIntegrationAuthorizationCode(ctx, integration.ID, codeHash[:], stateHash[:], time.Now())
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, e.WithStatus(http.StatusNotFound, errors.New("authorization not found"))
	}
	if err != nil {
		a.log.Error("integration authorization redemption failed", "err", err)
		return nil, e.WithStatus(http.StatusInternalServerError, errors.New("could not redeem authorization"))
	}
	return &protobuf.RedeemIntegrationAuthorizationResponse{GrantId: uint32(grantID), CourseId: uint32(courseID)}, nil
}

func (a *API) GetIntegrationGrant(ctx context.Context, req *protobuf.GetIntegrationGrantRequest) (*protobuf.GetIntegrationGrantResponse, error) {
	if req.GetGrantId() == 0 {
		return nil, e.WithStatus(http.StatusBadRequest, errors.New("grant_id must be positive"))
	}
	integration, err := a.getCurrentIntegration(ctx)
	if err != nil {
		return nil, err
	}
	course, err := a.dao.IntegrationGrantDao.GetIntegrationGrantCourse(ctx, uint(req.GetGrantId()), integration.ID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, e.WithStatus(http.StatusNotFound, errors.New("grant not found"))
	}
	if err != nil {
		a.log.Error("integration grant lookup failed", "err", err)
		return nil, e.WithStatus(http.StatusInternalServerError, errors.New("could not read integration grant"))
	}
	return &protobuf.GetIntegrationGrantResponse{
		CourseId: uint32(course.ID), Name: course.Name, Slug: course.Slug, Visibility: course.Visibility,
	}, nil
}

func (a *API) RevokeIntegrationGrant(ctx context.Context, req *protobuf.RevokeIntegrationGrantRequest) (*emptypb.Empty, error) {
	if req.GetGrantId() == 0 {
		return nil, e.WithStatus(http.StatusBadRequest, errors.New("grant_id must be positive"))
	}
	integration, err := a.getCurrentIntegration(ctx)
	if err != nil {
		return nil, err
	}
	course, err := a.dao.IntegrationGrantDao.RevokeIntegrationGrantForIntegration(ctx, uint(req.GetGrantId()), integration.ID)
	if err != nil {
		a.log.Error("integration grant revocation failed", "err", err)
		return nil, e.WithStatus(http.StatusInternalServerError, errors.New("could not revoke integration grant"))
	}
	if course.ID != 0 {
		// No user acted; the integration is named in the message instead.
		message := fmt.Sprintf("%s:'%s' revoke integration grant: %d by integration '%s' (%d)", course.Name, course.Slug, req.GetGrantId(), integration.Name, integration.ID)
		if err := a.dao.AuditDao.Create(&model.Audit{Message: message, Type: model.AuditCourseEdit}); err != nil {
			a.log.Error("can't write audit entry", "err", err, "message", message)
		}
	}
	return &emptypb.Empty{}, nil
}

// GetIntegrationRecording answers for one version, the requested one or else the one
// the watch page starts with when the viewer chose none (see
// integrationRecordingVersion). It hands out that playlist signed without a user like
// for an anonymous viewer of a public course: the grant is the permission here.
func (a *API) GetIntegrationRecording(ctx context.Context, req *protobuf.GetIntegrationRecordingRequest) (*protobuf.GetIntegrationRecordingResponse, error) {
	if req.GetGrantId() == 0 || req.GetStreamId() == 0 {
		return nil, e.WithStatus(http.StatusBadRequest, errors.New("grant_id and stream_id must be positive"))
	}
	integration, err := a.getCurrentIntegration(ctx)
	if err != nil {
		return nil, err
	}
	stream, err := a.dao.IntegrationGrantDao.GetIntegrationGrantStream(ctx, uint(req.GetGrantId()), integration.ID, uint(req.GetStreamId()))
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, e.WithStatus(http.StatusNotFound, errors.New("recording not found"))
	}
	if err != nil {
		a.log.Error("integration recording lookup failed", "err", err)
		return nil, e.WithStatus(http.StatusInternalServerError, errors.New("could not read recording"))
	}
	version, playlist, err := a.integrationRecordingVersion(ctx, &stream, req.StreamVersion)
	if err != nil {
		a.log.Error("integration recording course lookup failed", "err", err, "stream", stream.ID)
		return nil, e.WithStatus(http.StatusInternalServerError, errors.New("could not read recording"))
	}
	if req.StreamVersion != nil && playlist == "" {
		return nil, e.WithStatus(http.StatusNotFound, errors.New("recording version not found"))
	}
	response := &protobuf.GetIntegrationRecordingResponse{}
	progress, err := a.dao.StreamsDao.GetTranscodingProgressByVersion(version, stream.ID)
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		// No conversion under way and none failed.
	case err != nil:
		a.log.Error("integration recording progress lookup failed", "err", err, "stream", stream.ID)
		return nil, e.WithStatus(http.StatusInternalServerError, errors.New("could not read recording"))
	case progress.Failed:
		response.Failed, response.Error = true, progress.Error
	default:
		response.Converting, response.Progress = true, int32(progress.Progress)
	}
	if stream.Recording && playlist != "" {
		// Sign only the chosen playlist; the other versions are not handed out.
		chosen := model.Stream{Model: stream.Model, CourseID: stream.CourseID}
		*playlistOf(&chosen, version) = playlist
		if err := signPlaylists(&chosen, nil, false); err != nil {
			// The edge refuses unsigned playlists, so an unsigned one is of no use.
			a.log.Error("can't sign integration playlist", "err", err, "stream", stream.ID)
			return nil, e.WithStatus(http.StatusInternalServerError, errors.New("could not sign playlist"))
		}
		response.PlaylistUrl = *playlistOf(&chosen, version)
	}
	return response, nil
}

// integrationRecordingVersion picks the requested version, or else the version the
// watch page starts with when the viewer chose none: the first of combined,
// presentation and camera that the course records in the lecture hall and that has a
// playlist (as watchSources). A hall recorded as presentation or camera only has no
// combined version. Without any playlist yet it is the first version the hall records,
// whose conversion is the one to report; without a hall that is the combined version.
// A requested version the hall does not record has no playlist.
func (a *API) integrationRecordingVersion(ctx context.Context, s *model.Stream, requested *protobuf.VideoType) (model.StreamVersion, string, error) {
	mode := model.SourceModeCOMB
	if s.LectureHallID != 0 {
		course, err := a.dao.CoursesDao.GetCourseById(ctx, s.CourseID)
		if err != nil {
			return "", "", err
		}
		mode = course.GetSourceModeForLectureHall(s.LectureHallID)
	}
	versions := []model.StreamVersion{model.COMB, model.PRES, model.CAM}
	switch mode {
	case model.SourceModePRESOnly:
		versions = []model.StreamVersion{model.PRES}
	case model.SourceModeCAMOnly:
		versions = []model.StreamVersion{model.CAM}
	}
	if requested != nil {
		v := model.StreamVersion(requested.String())
		if !slices.Contains(versions, v) {
			return v, "", nil
		}
		versions = []model.StreamVersion{v}
	}
	for _, v := range versions {
		if url := *playlistOf(s, v); url != "" {
			return v, url, nil
		}
	}
	return versions[0], "", nil
}

// playlistOf is the field holding the playlist of a version.
func playlistOf(s *model.Stream, v model.StreamVersion) *string {
	switch v {
	case model.PRES:
		return &s.PlaylistUrlPRES
	case model.CAM:
		return &s.PlaylistUrlCAM
	default:
		return &s.PlaylistUrl
	}
}
