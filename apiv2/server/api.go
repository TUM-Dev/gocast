package apiv2

//go:generate ./generate.sh

import (
	"context"
	"embed"
	"log/slog"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/reflection"
	"google.golang.org/protobuf/types/known/emptypb"
	"gorm.io/gorm"

	protobuf "github.com/TUM-Dev/gocast/apiv2/protobuf/server"
	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/pkg/realtimehub"
	"github.com/TUM-Dev/gocast/tools"
	"github.com/TUM-Dev/gocast/tools/bot"
	"github.com/TUM-Dev/gocast/voice-service/pb"
)

// API is the grpc server for the v2 api. It implements the API's services, which are
// split for grouping rather than to be deployed apart: one process, one gateway mux,
// one set of interceptors.
type API struct {
	db  *gorm.DB
	dao dao.DaoWrapper
	log *slog.Logger

	// Reaching the lecture hall cameras, for the administration endpoints that move
	// one to a preset or photograph it. Optional: see WithCamService.
	cams           CamService
	presetImageDir string

	// TUMOnline, for creating courses. Optional: see WithTUMOnline.
	tumOnline TUMOnline

	// A lecture's content: section thumbnails, subtitles and where uploads go. All
	// optional: see WithSectionImages, WithSubtitleGenerator and WithMassStorage.
	sectionImages SectionImages
	// Regenerating a VOD's own large thumbnail, as opposed to a section's; see
	// WithPostProcessing. Optional: nil means RedoCoursePostProcessing answers 503.
	postProcessing PostProcessing
	// Where an uploaded recording is converted; see WithUploads. Optional: nil means
	// uploading a lecture's media answers 503.
	uploads Uploads
	// Courses, lectures and subtitles by text; see WithSearch.
	search        Search
	subtitles     pb.SubtitleGeneratorClient
	subtitlesAuth string
	massStorage   string
	// Where the runner's VOD directories are mounted, and the dry run kept for paging;
	// see WithVODStorage and ReapDeletedMedia.
	vodStorage string
	// The host whose /vod/ URLs point into vodStorage. Empty: taken from vodURLTemplate.
	vodEdgeHost string
	reapCache   reapCache
	// Pushes per-stream events to the realtime socket. New gives every API one of its
	// own; WithRealtimeHub shares it with the producers outside this package.
	hub realtimehub.Hub

	// The watch page: who reacted lately, who reported what, how a report reaches the
	// operators (nil: Matrix, tests replace it), and the optional means to end a live
	// stream and to search one recording's subtitles; see WithStreamEnder and
	// WithSubtitleSearch.
	reactions      reactionStats
	issues         issueLimiter
	sendAlert      func(bot.AlertMessage) error
	streamEnder    StreamEnder
	subtitleSearch tools.MeiliSearchInterface
	// Starts self streams for the mediamtx publish webhook; see WithSelfStreamStarter.
	selfStreams SelfStreamStarter

	protobuf.UnimplementedMetaServiceServer
	protobuf.UnimplementedIntegrationServiceServer
	protobuf.UnimplementedUserServiceServer
	protobuf.UnimplementedCourseServiceServer
	protobuf.UnimplementedStreamServiceServer
	protobuf.UnimplementedAdminServiceServer
	protobuf.UnimplementedChatServiceServer
}

// Option configures an API beyond what it needs to exist. What is optional here is
// what the API can serve without: an API with no camera service still answers every
// endpoint that does not touch a camera.
type Option func(*API)

// New creates a new API and assigns the given db and a logger
func New(db *gorm.DB, opts ...Option) *API {
	log := slog.With("apiVersion", "2")
	a := &API{
		db:  db,
		dao: dao.NewDaoWrapper(),
		log: log,
	}

	for _, opt := range opts {
		opt(a)
	}

	if a.hub == nil {
		a.hub = realtimehub.NewMemory()
	}

	return a
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

// Handler serves everything under /api/v2/: the gateway, which turns the REST surface
// into calls on the RPC server, and beside it the endpoints that cannot be RPCs.
func (a *API) Handler() http.Handler {
	// setup muxing
	gateway := runtime.NewServeMux()
	// DEPRECATED: opts := []grpc.DialOption{grpc.WithInsecure()}
	opts := []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())}

	// Every service registers onto the same mux, so the REST surface stays one flat
	// set of paths however the services are cut.
	for _, svc := range services {
		if err := svc.gateway(context.Background(), gateway, ":8081", opts); err != nil {
			a.log.With("err", err, "service", svc.desc.ServiceName).Error("can't register grpc handler")
			os.Exit(1)
		}
	}
	rpc := http.StripPrefix("/api/v2", gateway)

	mux := http.NewServeMux()
	// Whatever is not claimed below is an RPC.
	mux.Handle("/api/v2/", rpc)
	mux.HandleFunc("GET /api/v2/docs", a.handleDocs)
	mux.HandleFunc("GET /api/v2/docs/", a.handleDocs)
	// Beside the gateway rather than through it: it reads a cookie, which the
	// gateway deliberately abstracts away.
	mux.HandleFunc("/api/v2/auth/token", a.handleAuthToken)
	// A websocket; the gateway cannot serve one.
	mux.HandleFunc(RealtimePath, a.serveRealtime)

	// Beside the gateway too, each for what an RPC cannot do. A pattern matches any
	// text where the path has an id, so a request whose id is no number is handed on
	// to the gateway, which answers it as it answers any other unknown path.
	//
	// It sets a session cookie.
	mux.HandleFunc(impersonatePath, func(w http.ResponseWriter, r *http.Request) {
		id, ok := impersonateIDOf(r)
		if !ok {
			rpc.ServeHTTP(w, r)
			return
		}
		a.handleImpersonate(w, r, id)
	})
	// It answers text/calendar from a path with a file name.
	mux.HandleFunc(courseCalendarPath, a.handleCourseCalendar)
	// It cannot take a multipart body.
	for _, kind := range []string{uploadAttachment, uploadThumbnail, uploadMedia} {
		mux.HandleFunc(lectureUploadPathPrefix+kind, func(w http.ResponseWriter, r *http.Request) {
			route, ok := lectureUploadRouteOf(r, kind)
			if !ok {
				rpc.ServeHTTP(w, r)
				return
			}
			a.handleLectureUpload(w, r, route)
		})
	}
	// A multipart body, and a browser's CORS preflight.
	mux.HandleFunc(integrationUploadPath, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost && r.Method != http.MethodOptions {
			rpc.ServeHTTP(w, r)
			return
		}
		a.handleIntegrationUpload(w, r, r.PathValue("ticket"))
	})
	// It streams a file, with Range, which the gateway's unary responses cannot.
	mux.HandleFunc(streamFilePath, func(w http.ResponseWriter, r *http.Request) {
		route, ok := streamFileRouteOf(r)
		if !ok {
			rpc.ServeHTTP(w, r)
			return
		}
		a.handleStreamFile(w, r, route)
	})
	// The v2 addresses of the routes of the retired v1 API; see RegisterLegacyRoutes.
	mux.HandleFunc(selfStreamPublishPath, a.handleSelfStreamPublish)
	mux.HandleFunc(tokenProxyPath, a.handleTokenProxy)
	mux.HandleFunc(lectureHallSchedule, a.handleLectureHallSchedule)
	return mux
}

//go:embed docs
var openApiJson embed.FS

// handleDocs serves the openapi.json file and the swagger ui
func (a *API) handleDocs(w http.ResponseWriter, r *http.Request) {
	httpFs := http.FS(openApiJson)
	fileServer := http.FileServer(httpFs)
	http.StripPrefix("/api/v2", fileServer).ServeHTTP(w, r)
}

// HealthCheck returns ok
func (a *API) HealthCheck(ctx context.Context, req *emptypb.Empty) (*protobuf.HealthCheckResponse, error) {
	return &protobuf.HealthCheckResponse{Status: "OK"}, nil
}
