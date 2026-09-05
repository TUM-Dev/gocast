package apiv2

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"net"
	"net/http"
	"net/url"
	"strings"

	"google.golang.org/protobuf/types/known/emptypb"

	e "github.com/TUM-Dev/gocast/apiv2/errors"
	protobuf "github.com/TUM-Dev/gocast/apiv2/protobuf/server"
	"github.com/TUM-Dev/gocast/model"
)

// Every RPC here requires PermAdministerServer through its policy in services.go.

func (a *API) CreateIntegration(ctx context.Context, req *protobuf.CreateIntegrationRequest) (*protobuf.CreateIntegrationResponse, error) {
	name := strings.TrimSpace(req.GetName())
	returnURL := strings.TrimSpace(req.GetReturnUrl())
	if name == "" || len(name) > 100 || len(returnURL) > 2048 || !validIntegrationReturnURL(returnURL) {
		return nil, e.WithStatus(http.StatusBadRequest, errors.New("enter a name and an HTTPS return URL; HTTP is allowed for loopback development"))
	}
	key, hash, err := newIntegrationKey()
	if err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}
	integration := model.Integration{Name: name, ReturnURL: returnURL, APIKeyHash: hash}
	if err := a.dao.IntegrationDao.CreateIntegration(&integration); err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}
	return &protobuf.CreateIntegrationResponse{
		Id: uint32(integration.ID), Name: integration.Name, ReturnUrl: integration.ReturnURL, ApiKey: key,
	}, nil
}

func (a *API) RotateIntegrationKey(ctx context.Context, req *protobuf.RotateIntegrationKeyRequest) (*protobuf.RotateIntegrationKeyResponse, error) {
	if req.GetId() == 0 {
		return nil, e.WithStatus(http.StatusBadRequest, errors.New("invalid integration"))
	}
	key, hash, err := newIntegrationKey()
	if err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}
	if err := a.dao.IntegrationDao.SetIntegrationAPIKey(uint(req.GetId()), hash); err != nil {
		return nil, e.FromGorm(err, "integration not found")
	}
	return &protobuf.RotateIntegrationKeyResponse{ApiKey: key}, nil
}

func (a *API) RevokeIntegrationKey(ctx context.Context, req *protobuf.RevokeIntegrationKeyRequest) (*emptypb.Empty, error) {
	if req.GetId() == 0 {
		return nil, e.WithStatus(http.StatusBadRequest, errors.New("invalid integration"))
	}
	if err := a.dao.IntegrationDao.SetIntegrationAPIKey(uint(req.GetId()), nil); err != nil {
		return nil, e.FromGorm(err, "integration not found")
	}
	return &emptypb.Empty{}, nil
}

func newIntegrationKey() (string, []byte, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", nil, err
	}
	hash := sha256.Sum256(raw)
	return base64.RawURLEncoding.EncodeToString(raw), hash[:], nil
}

func validIntegrationReturnURL(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || !u.IsAbs() || u.Hostname() == "" || u.User != nil || u.Fragment != "" {
		return false
	}
	if u.Scheme == "https" {
		return true
	}
	host := strings.ToLower(u.Hostname())
	ip := net.ParseIP(host)
	return u.Scheme == "http" && (host == "localhost" || strings.HasSuffix(host, ".localhost") || ip != nil && ip.IsLoopback())
}
