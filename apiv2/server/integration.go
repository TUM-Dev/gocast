package apiv2

import (
	"context"

	"google.golang.org/protobuf/types/known/emptypb"

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
