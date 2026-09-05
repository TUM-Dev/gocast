package apiv2

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"gorm.io/gorm"

	protobuf "github.com/TUM-Dev/gocast/apiv2/protobuf/server"
	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/mock_dao"
	"github.com/TUM-Dev/gocast/model"
)

func integrationBearer(raw []byte) context.Context {
	key := base64.RawURLEncoding.EncodeToString(raw)
	return metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+key))
}

func TestIntegrationAuthentication(t *testing.T) {
	raw := bytes.Repeat([]byte{7}, 32)
	hash := sha256.Sum256(raw)
	next := bytes.Repeat([]byte{8}, 32)
	nextHash := sha256.Sum256(next)
	integration := model.Integration{ID: 4, Name: "Course portal", ReturnURL: "https://example/callback"}
	ctrl := gomock.NewController(t)
	integrationDao := mock_dao.NewMockIntegrationDao(ctrl)
	gomock.InOrder(
		integrationDao.EXPECT().GetIntegrationByAPIKeyHash(gomock.Any(), hash[:]).Return(integration, nil),
		integrationDao.EXPECT().GetIntegrationByAPIKeyHash(gomock.Any(), hash[:]).Return(model.Integration{}, gorm.ErrRecordNotFound),
		integrationDao.EXPECT().GetIntegrationByAPIKeyHash(gomock.Any(), nextHash[:]).Return(integration, nil),
		integrationDao.EXPECT().GetIntegrationByAPIKeyHash(gomock.Any(), nextHash[:]).Return(model.Integration{}, nil),
		integrationDao.EXPECT().GetIntegrationByAPIKeyHash(gomock.Any(), nextHash[:]).Return(model.Integration{}, errors.New("database unavailable")),
	)
	api := &API{dao: dao.DaoWrapper{IntegrationDao: integrationDao}, log: slog.Default()}
	info := &grpc.UnaryServerInfo{FullMethod: method(&protobuf.MetaService_ServiceDesc, "getIntegration")}
	_, err := api.resolveCaller(integrationBearer(raw), nil, info, func(ctx context.Context, _ any) (any, error) {
		return api.authorize(ctx, nil, info, func(ctx context.Context, _ any) (any, error) {
			resolved := ctx.Value(callerKey{}).(*caller)
			require.NotNil(t, resolved.integration)
			assert.Nil(t, resolved.user)
			assert.NoError(t, resolved.err)
			response, callErr := api.GetIntegration(ctx, &emptypb.Empty{})
			require.NoError(t, callErr)
			assert.Equal(t, uint32(4), response.Id)
			assert.Equal(t, "https://example/callback", response.ReturnUrl)
			return nil, nil
		})
	})
	require.NoError(t, err)

	_, err = api.resolveIntegration(integrationBearer(raw))
	assert.Equal(t, codes.Unauthenticated, status.Code(err), "old key remained valid after rotation")
	_, err = api.resolveIntegration(integrationBearer(next))
	require.NoError(t, err)
	_, err = api.resolveIntegration(integrationBearer(next))
	assert.Equal(t, codes.Unauthenticated, status.Code(err), "revoked key remained valid")

	_, err = api.resolveIntegration(integrationBearer([]byte("short")))
	assert.Equal(t, codes.Unauthenticated, status.Code(err))
	_, err = api.resolveIntegration(integrationBearer(next))
	assert.Equal(t, codes.Unknown, status.Code(err))
	assert.Equal(t, "could not authenticate integration", status.Convert(err).Message())
}

func TestIntegrationCredentialsStaySeparateFromUserCredentials(t *testing.T) {
	api := &API{log: slog.Default()}
	integrationMethod := &grpc.UnaryServerInfo{FullMethod: method(&protobuf.MetaService_ServiceDesc, "getIntegration")}
	userMethod := &grpc.UnaryServerInfo{FullMethod: method(&protobuf.UserService_ServiceDesc, "getUser")}
	for _, test := range []struct {
		name string
		ctx  context.Context
		info *grpc.UnaryServerInfo
	}{
		{"session cookie on integration method", metadata.NewIncomingContext(context.Background(), metadata.Pairs("grpcgateway-cookie", "jwt=session")), integrationMethod},
		{"human JWT on integration method", metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxIn0.signature")), integrationMethod},
		{"integration key on user method", integrationBearer(bytes.Repeat([]byte{7}, 32)), userMethod},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := api.resolveCaller(test.ctx, nil, test.info, func(ctx context.Context, req any) (any, error) {
				return api.authorize(ctx, req, test.info, func(context.Context, any) (any, error) {
					t.Fatal("unexpected access to protected method")
					return nil, nil
				})
			})
			assert.Equal(t, codes.Unauthenticated, status.Code(err))
		})
	}
}
