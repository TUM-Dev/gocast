package apiv2

import (
	"context"
	"crypto/sha256"
	"errors"
	"net/http"
	"time"

	uuid "github.com/satori/go.uuid"
	"google.golang.org/protobuf/types/known/emptypb"
	"gorm.io/gorm"

	e "github.com/TUM-Dev/gocast/apiv2/errors"
	protobuf "github.com/TUM-Dev/gocast/apiv2/protobuf/server"
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
