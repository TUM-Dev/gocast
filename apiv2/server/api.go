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

// API is the grpc server for the v2 api. It implements all four services, which are
// split for grouping rather than to be deployed apart: one process, one gateway mux,
// one set of interceptors.
type API struct {
	db  *gorm.DB
	dao dao.DaoWrapper
	log *slog.Logger

	protobuf.UnimplementedMetaServiceServer
	protobuf.UnimplementedUserServiceServer
	protobuf.UnimplementedCourseServiceServer
	protobuf.UnimplementedStreamServiceServer
	protobuf.UnimplementedAdminServiceServer
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
	grpcServer := grpc.NewServer(
		grpc.KeepaliveParams(keepalive.ServerParameters{
			MaxConnectionIdle:     time.Minute,
			MaxConnectionAge:      time.Minute,
			MaxConnectionAgeGrace: time.Second * 5,
			Time:                  time.Minute * 10,
			Timeout:               time.Second * 20,
		}),
		a.interceptors(),
	)

	for _, svc := range services {
		svc.register(grpcServer, a)
	}

	// Pre-creates the series for every method, so a method that has not been called
	// yet reads as zero rather than as a gap.
	serverMetrics.InitializeMetrics(grpcServer)

	reflection.Register(grpcServer)
	return grpcServer.Serve(lis)
}

// Proxy returns a gin handler that proxies requests to the grpc gateway server
func (a *API) Proxy() func(c *gin.Context) {
	// setup muxing
	mux := runtime.NewServeMux()
	// DEPRECATED: opts := []grpc.DialOption{grpc.WithInsecure()}
	opts := []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())}

	// Every service registers onto the same mux, so the REST surface stays one flat
	// set of paths however the services are cut.
	for _, svc := range services {
		if err := svc.gateway(context.Background(), mux, ":8081", opts); err != nil {
			a.log.With("err", err, "service", svc.desc.ServiceName).Error("can't register grpc handler")
			os.Exit(1)
		}
	}

	// actual proxy method forwards the request to the grpc gateway server
	return func(c *gin.Context) {
		if c.Request.Method == http.MethodGet && strings.HasPrefix(c.Request.URL.Path, "/api/v2/docs") {
			a.handleDocs(c)
			return
		}
		// Beside the gateway rather than through it: it reads a cookie, which the
		// gateway deliberately abstracts away.
		if c.Request.URL.Path == "/api/v2/auth/token" {
			a.handleAuthToken(c)
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
