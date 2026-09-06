package apiv2

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"net/http"
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
