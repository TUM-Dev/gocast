package apiv2

//go:generate ./generate.sh

import (
	"context"
	"embed"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/reflection"
	"google.golang.org/protobuf/types/known/emptypb"
	"gorm.io/gorm"

	protobuf "github.com/TUM-Dev/gocast/apiv2/protobuf/server"
	"github.com/TUM-Dev/gocast/dao"
)

// API is the grpc server for the v2 api
type API struct {
	db  *gorm.DB
	dao dao.DaoWrapper
	log *slog.Logger

	protobuf.UnimplementedAPIServer
	protobuf.UnimplementedIntegrationServiceServer
}

// New creates a new API and assigns the given db and a logger
func New(db *gorm.DB) *API {
	log := slog.With("apiVersion", "2")
	return &API{
		db:  db,
		dao: dao.NewDaoWrapper(),
		log: log,
	}
}

// Run starts the grpc server on port 12544 and the grpc gateway on ::8081/api/v2
func (a *API) Run(lis net.Listener) error {
	a.log.Info("Running")
	grpcServer := grpc.NewServer(grpc.KeepaliveParams(keepalive.ServerParameters{
		MaxConnectionIdle:     time.Minute,
		MaxConnectionAge:      time.Minute,
		MaxConnectionAgeGrace: time.Second * 5,
		Time:                  time.Minute * 10,
		Timeout:               time.Second * 20,
	}))

	protobuf.RegisterAPIServer(grpcServer, a)
	protobuf.RegisterIntegrationServiceServer(grpcServer, a)
	reflection.Register(grpcServer)
	return grpcServer.Serve(lis)
}

// integrationHeaderMatcher is the shared gRPC-gateway incoming-header matcher
// used by the single gateway mux serving both API and IntegrationService.
//
// It maps:
//   - Authorization  → "authorization"       (bearer token for service-account auth;
//     without this the default mapper renames it to grpcgateway-authorization)
//   - X-On-Behalf-Of → "x-on-behalf-of"      (on-behalf-of LRZ ID for integration RPCs)
//   - All others     → runtime.DefaultHeaderMatcher (preserves grpcgateway-cookie
//     so existing cookie-based auth continues to work)
func integrationHeaderMatcher(key string) (string, bool) {
	switch {
	case strings.EqualFold(key, "Authorization"):
		return "authorization", true
	case strings.EqualFold(key, "X-On-Behalf-Of"):
		return "x-on-behalf-of", true
	default:
		return runtime.DefaultHeaderMatcher(key)
	}
}

// Proxy returns a gin handler that proxies requests to the grpc gateway server
func (a *API) Proxy() func(c *gin.Context) {
	// setup muxing — one shared mux for both services so the custom header
	// matcher and cookie auth apply uniformly.
	mux := runtime.NewServeMux(runtime.WithIncomingHeaderMatcher(integrationHeaderMatcher))
	// DEPRECATED: opts := []grpc.DialOption{grpc.WithInsecure()}
	opts := []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())}
	err := protobuf.RegisterAPIHandlerFromEndpoint(context.Background(), mux, ":8081", opts)
	if err != nil {
		a.log.With("err", err).Error("can't register grpc handler")
		os.Exit(1)
	}
	if err := protobuf.RegisterIntegrationServiceHandlerFromEndpoint(context.Background(), mux, ":8081", opts); err != nil {
		a.log.With("err", err).Error("can't register integration grpc gateway handler")
		os.Exit(1)
	}

	// actual proxy method forwards the request to the grpc gateway server
	return func(c *gin.Context) {
		if c.Request.Method == http.MethodGet && strings.HasPrefix(c.Request.URL.Path, "/api/v2/docs") {
			a.handleDocs(c)
			return
		}
		http.StripPrefix("/api/v2", mux).ServeHTTP(c.Writer, c.Request)
	}
}

//go:embed docs
var openApiJson embed.FS

// handleDocs serves the openapi.json file and the swagger ui
func (a *API) handleDocs(c *gin.Context) {
	httpFs := http.FS(openApiJson)
	fileServer := http.FileServer(httpFs)
	http.StripPrefix("/api/v2", fileServer).ServeHTTP(c.Writer, c.Request)
}

// HealthCheck returns ok
func (a *API) HealthCheck(ctx context.Context, req *emptypb.Empty) (*protobuf.HealthCheckResponse, error) {
	return &protobuf.HealthCheckResponse{Status: "OK"}, nil
}
