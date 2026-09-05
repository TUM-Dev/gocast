package apiv2

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"testing"

	"go.uber.org/mock/gomock"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"

	protobuf "github.com/TUM-Dev/gocast/apiv2/protobuf/server"
	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/mock_dao"
	"github.com/TUM-Dev/gocast/model"
)

func TestCreateIntegration(t *testing.T) {
	var saved model.Integration
	integrations := mock_dao.NewMockIntegrationDao(gomock.NewController(t))
	integrations.EXPECT().CreateIntegration(gomock.Any()).DoAndReturn(func(integration *model.Integration) error {
		integration.ID = 7
		saved = *integration
		return nil
	})
	api := &API{dao: dao.DaoWrapper{IntegrationDao: integrations}}
	response, err := api.CreateIntegration(context.Background(), &protobuf.CreateIntegrationRequest{
		Name: " Course portal ", ReturnUrl: " https://portal.example/callback ",
	})
	if err != nil {
		t.Fatal(err)
	}
	if response.Id != 7 || response.Name != "Course portal" || response.ReturnUrl != "https://portal.example/callback" {
		t.Fatalf("unexpected integration: id=%d, name=%q, returnUrl=%q", response.Id, response.Name, response.ReturnUrl)
	}
	checkIntegrationKey(t, response.ApiKey, saved.APIKeyHash)
}

func checkIntegrationKey(t *testing.T, key string, storedHash []byte) {
	t.Helper()
	raw, err := base64.RawURLEncoding.DecodeString(key)
	if err != nil || len(raw) != 32 {
		t.Fatal("key must encode 32 random bytes")
	}
	hash := sha256.Sum256(raw)
	if !bytes.Equal(hash[:], storedHash) {
		t.Fatal("stored hash does not match the returned key")
	}
}

func TestCreateIntegrationValidation(t *testing.T) {
	api := &API{}
	for raw, valid := range map[string]bool{
		"https://portal.example/callback": true, "http://localhost:8080/callback": true,
		"http://127.0.0.1/callback": true, "http://app.localhost/callback": true,
		"http://portal.example/callback": false, "https://portal.example/#fragment": false,
		"https://portal.example/?next=/x#fragment": false, "https://:443/callback": false,
		"//portal.example/callback": false, "https://user@portal.example/callback": false,
	} {
		if validIntegrationReturnURL(raw) != valid {
			t.Errorf("validIntegrationReturnURL(%q) should be %v", raw, valid)
		}
		if !valid {
			_, err := api.CreateIntegration(context.Background(), &protobuf.CreateIntegrationRequest{Name: "Portal", ReturnUrl: raw})
			if status.Code(err) != codes.InvalidArgument {
				t.Errorf("invalid URL %q: %v", raw, err)
			}
		}
	}
	_, err := api.CreateIntegration(context.Background(), &protobuf.CreateIntegrationRequest{ReturnUrl: "https://portal.example/callback"})
	if status.Code(err) != codes.InvalidArgument {
		t.Errorf("missing name: %v", err)
	}
}

func TestIntegrationKeyChanges(t *testing.T) {
	var storedHash []byte
	integrations := mock_dao.NewMockIntegrationDao(gomock.NewController(t))
	integrations.EXPECT().SetIntegrationAPIKey(uint(7), gomock.Any()).DoAndReturn(func(_ uint, hash []byte) error {
		storedHash = bytes.Clone(hash)
		return nil
	})
	api := &API{dao: dao.DaoWrapper{IntegrationDao: integrations}}
	response, err := api.RotateIntegrationKey(context.Background(), &protobuf.RotateIntegrationKeyRequest{Id: 7})
	if err != nil {
		t.Fatal(err)
	}
	checkIntegrationKey(t, response.ApiKey, storedHash)
	integrations.EXPECT().SetIntegrationAPIKey(uint(7), nil).Return(nil)
	if _, err := api.RevokeIntegrationKey(context.Background(), &protobuf.RevokeIntegrationKeyRequest{Id: 7}); err != nil {
		t.Fatal(err)
	}
}

func TestIntegrationAdminErrors(t *testing.T) {
	integrations := mock_dao.NewMockIntegrationDao(gomock.NewController(t))
	api := &API{dao: dao.DaoWrapper{IntegrationDao: integrations}}
	integrations.EXPECT().CreateIntegration(gomock.Any()).Return(errors.New("database unavailable"))
	_, err := api.CreateIntegration(context.Background(), &protobuf.CreateIntegrationRequest{Name: "Portal", ReturnUrl: "https://portal.example/callback"})
	if status.Code(err) != codes.Unknown {
		t.Errorf("failed create: %v", err)
	}
	for _, call := range []func(uint32) error{
		func(id uint32) error {
			_, err := api.RotateIntegrationKey(context.Background(), &protobuf.RotateIntegrationKeyRequest{Id: id})
			return err
		},
		func(id uint32) error {
			_, err := api.RevokeIntegrationKey(context.Background(), &protobuf.RevokeIntegrationKeyRequest{Id: id})
			return err
		},
	} {
		if err := call(0); status.Code(err) != codes.InvalidArgument {
			t.Errorf("zero ID: %v", err)
		}
		for _, dbErr := range []error{gorm.ErrRecordNotFound, errors.New("database unavailable")} {
			integrations.EXPECT().SetIntegrationAPIKey(uint(7), gomock.Any()).Return(dbErr)
			want := codes.Unknown
			if errors.Is(dbErr, gorm.ErrRecordNotFound) {
				want = codes.NotFound
			}
			if err := call(7); status.Code(err) != want {
				t.Errorf("code = %v, want %v", status.Code(err), want)
			}
		}
	}
}

func TestIntegrationAdminPolicies(t *testing.T) {
	api := &API{}
	for _, name := range []string{"createIntegration", "rotateIntegrationKey", "revokeIntegrationKey"} {
		for _, tc := range []struct {
			user *model.User
			code codes.Code
		}{
			{nil, codes.Unauthenticated},
			{&model.User{Role: model.StudentType}, codes.PermissionDenied},
			{&model.User{Role: model.LecturerType}, codes.PermissionDenied},
			{&model.User{Role: model.AdminType}, codes.OK},
		} {
			resolved := &caller{user: tc.user}
			if tc.user == nil {
				resolved.err = ErrNoCredentials
			}
			ctx := context.WithValue(context.Background(), callerKey{}, resolved)
			called := false
			_, err := api.authorize(ctx, nil, &grpc.UnaryServerInfo{FullMethod: method(&protobuf.AdminService_ServiceDesc, name)}, func(context.Context, any) (any, error) {
				called = true
				return nil, nil
			})
			if status.Code(err) != tc.code || called != (tc.code == codes.OK) {
				t.Errorf("%s: code=%v called=%v, want code=%v", name, status.Code(err), called, tc.code)
			}
		}
	}
}
