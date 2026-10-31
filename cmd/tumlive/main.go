package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/http/pprof"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/dgraph-io/ristretto/v2"
	slogGorm "github.com/orandin/slog-gorm"
	"github.com/soheilhy/cmux"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"github.com/tum-dev/gocast/runner/pkg/tracing"

	apiv2 "github.com/TUM-Dev/gocast/apiv2/server"
	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/model"
	"github.com/TUM-Dev/gocast/pkg/camera"
	"github.com/TUM-Dev/gocast/pkg/livepreview"
	"github.com/TUM-Dev/gocast/pkg/livestate"
	"github.com/TUM-Dev/gocast/pkg/postprocessing"
	"github.com/TUM-Dev/gocast/pkg/realtimehub"
	"github.com/TUM-Dev/gocast/pkg/runner_manager"
	"github.com/TUM-Dev/gocast/pkg/search"
	"github.com/TUM-Dev/gocast/pkg/sectionimages"
	"github.com/TUM-Dev/gocast/pkg/uploads"
	"github.com/TUM-Dev/gocast/pkg/voicegrpc"
	"github.com/TUM-Dev/gocast/tools"
	"github.com/TUM-Dev/gocast/tools/tum"
	"github.com/TUM-Dev/gocast/voice-service/pb"
	"github.com/TUM-Dev/gocast/web"
)

func main() {
	testEnv := flag.Bool("test-env", false, "run against a copy of the production database: no cronjobs, no mailer, no Meilisearch settings sync, and no stream starts except for --stream-courses")
	streamCourses := flag.String("stream-courses", "", "comma-separated course IDs that may start streams in --test-env")
	flag.Parse()

	ctx := context.Background()
	policy, err := tools.NewTestEnvPolicy(*testEnv, *streamCourses)
	if err != nil {
		slog.With("err", err).ErrorContext(ctx, "invalid flags")
		os.Exit(2)
	}
	tools.TestEnv = policy
	err = run(ctx)
	if err != nil {
		slog.With("err", err).ErrorContext(ctx, "shutting down")
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	ctx, cancel := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer cancel()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})).With("service", "main")
	initAll(initializers)

	// Off unless OTEL_EXPORTER_OTLP_ENDPOINT is set, see tracing.Setup.
	shutdownTracing, err := tracing.Setup(ctx, "tumlive", VersionTag)
	if err != nil {
		logger.Error("tracing disabled", "err", err)
	} else {
		defer func() {
			flushCtx, stop := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
			defer stop()
			if err := shutdownTracing(flushCtx); err != nil {
				logger.Error("flushing traces failed", "err", err)
			}
		}()
	}

	web.VersionTag = VersionTag
	tools.VersionTag = VersionTag

	gormJSONLogger := slogGorm.New(
		slogGorm.WithSlowThreshold(500 * time.Millisecond),
	)

	db, err := gorm.Open(mysql.Open(fmt.Sprintf(
		"%s:%s@tcp(%s:%d)/%s?parseTime=true&loc=Local",
		tools.Cfg.Db.User,
		tools.Cfg.Db.Password,
		tools.Cfg.Db.Host,
		tools.Cfg.Db.Port,
		tools.Cfg.Db.Database),
	), &gorm.Config{
		PrepareStmt: true,
		Logger:      gormJSONLogger,
	})
	if err != nil {
		logger.Error("Error opening database", "err", err)
	}
	dao.DB = db

	err = dao.Migrator.RunBefore(db)
	if err != nil {
		return fmt.Errorf("before migration: %s", err)
	}

	err = db.AutoMigrate(
		&model.User{},
		&model.Course{},
		&model.Chat{},
		&model.RegisterLink{},
		&model.Silence{},
		&model.Stat{},
		&model.LectureHall{},
		&model.IngestServer{},
		&model.StreamName{},
		&model.Stream{},
		&model.CameraPreset{},
		&model.ServerNotification{},
		&model.File{},
		&model.StreamProgress{},
		&model.Token{},
		&model.Poll{},
		&model.PollOption{},
		&model.VideoSection{},
		&model.VideoSeekChunk{},
		&model.Notification{},
		&model.UserSetting{},
		&model.Audit{},
		&model.InfoPage{},
		&model.Bookmark{},
		&model.TranscodingProgress{},
		&model.ChatReaction{},
		&model.Subtitles{},
		&model.TranscodingFailure{},
		&model.Email{},
		&model.StreamReaction{},
		&model.Runner{},
		&model.StreamRunnerJob{},
		&model.Integration{},
		&model.IntegrationGrant{},
		&model.IntegrationAuthorizationCode{},
		&model.IntegrationRecording{},
	)
	if err != nil {
		return fmt.Errorf("migration: %w", err)
	}
	err = dao.Migrator.RunAfter(db)
	if err != nil {
		return fmt.Errorf("after migrate: %s", err)
	}

	cache, _ := ristretto.NewCache[string, any](&ristretto.Config[string, any]{
		NumCounters: 1e7,     // number of keys to track frequency of (10M).
		MaxCost:     1 << 30, // maximum cost of cache (1GB).
		BufferItems: 64,      // number of keys per Get buffer.
		Metrics:     true,
	})
	dao.Cache = cache

	// init cam service
	camAuths := make(map[model.CameraType]string)
	if tools.Cfg.Auths.CamAuth != "" {
		// todo: per-camera auths
		camAuths[model.Panasonic] = tools.Cfg.Auths.CamAuth
		camAuths[model.Axis] = tools.Cfg.Auths.CamAuth
	}
	if tools.Cfg.Auths.CamAuthSony != "" {
		camAuths[model.Sony_SRG_A40] = tools.Cfg.Auths.CamAuthSony
	}
	camService := camera.NewService(camAuths)

	// One hub for the process: the v2 realtime socket reads it, and the producers
	// (the runner's live flips, title and description edits) publish to it.
	hub := realtimehub.NewMemory()
	livestate.SetRealtimeHub(hub)

	opts := []runner_manager.Option{
		runner_manager.WithMassStorage(tools.Cfg.Paths.Mass),
		runner_manager.WithCamService(camService),
		runner_manager.WithLiveStateNotifier(livestate.NotifyViewersLiveState),
	}
	// What apiv2 requests subtitles through; nil without a voice service.
	var apiv2Subtitles pb.SubtitleGeneratorClient
	if tools.Cfg.VoiceService.Host != "" {
		voicegrpc.RunVoiceServiceReceiver(tools.Cfg.VoiceService.AuthToken)
		c, err := grpc.NewClient(fmt.Sprintf("%s:%s", tools.Cfg.VoiceService.Host, tools.Cfg.VoiceService.Port), grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			logger.Error("failed to connect to voice service", "err", err)
		} else {
			opts = append(opts, runner_manager.WithSubtitleClient(pb.NewSubtitleGeneratorClient(c), tools.Cfg.VoiceService.AuthToken))
			apiv2Subtitles = pb.NewSubtitleGeneratorClient(c)
		}
	}
	m := runner_manager.New(dao.NewDaoWrapper(), opts...)
	logger.Info("running runner manager")
	err = m.Run()
	if err != nil {
		logger.Error("Failed to start runner manager", "err", err)
	}

	if tools.TestEnv.Enabled {
		// The database is a copy of production: its email queue holds mails for real
		// users, and Meilisearch may be the real instance.
		logger.Warn("test environment: cronjobs, mailer and Meilisearch settings sync are disabled",
			"streamCourses", tools.TestEnv.StreamCourseIDs())
		// Keep the (empty) cron service so the admin cron RPCs list no jobs instead of panicking.
		tools.InitCronService()
	} else {
		// init meili search index settings
		go tools.NewMeiliExporter(dao.NewDaoWrapper()).SetIndexSettings()

		mailer := tools.NewMailer(dao.NewDaoWrapper(), tools.Cfg.Mail.MaxMailsPerMinute)
		go mailer.Run()

		initCron(logger, m, hub)
	}
	return serveHttp(ctx, m, camService, apiv2Subtitles, hub)
}

var VersionTag = "development"

type initializer func()

var initializers = []initializer{
	tools.LoadConfig,
	tools.InitBranding,
}

func initAll(initializers []initializer) {
	for _, init := range initializers {
		init()
	}
}

// newRouter mounts everything the web port serves. There is no compression here: the
// reverse proxy in front compresses.
//
// ServeMux panics when two patterns can match one request and neither is more
// specific, and the apiv2 and web routes only meet here; TestNewRouter registers them
// together so a conflict fails a test instead of the boot.
func newRouter(api2Client *apiv2.API) *tools.Router {
	router := tools.NewRouter()
	router.Use(tools.InitContext(dao.NewDaoWrapper()))
	router.Handle("/api/v2/", api2Client.Handler())
	api2Client.RegisterLegacyRoutes(router)
	web.ConfigRouter(router)
	return router
}

// serveHttp launches all http servers
func serveHttp(ctx context.Context, manager *runner_manager.Manager, camService *camera.Service, subtitles pb.SubtitleGeneratorClient, hub realtimehub.Hub) (err error) {
	if VersionTag != "development" {
		tools.CookieSecure = true
	}

	l, err := net.Listen("tcp", ":8081")
	if err != nil {
		return err
	}

	m := cmux.New(l)

	go func() {
		<-ctx.Done()
		m.Close()
	}()

	grpcl := m.MatchWithWriters(cmux.HTTP2MatchHeaderFieldSendSettings("content-type", "application/grpc"))
	httpl := m.Match(cmux.Any())

	// The same camera service and image directory v1 is given below, so a preset
	// photographed through either API lands in the same place and is served from it.
	api2Opts := []apiv2.Option{
		apiv2.WithCamService(camService),
		apiv2.WithPresetImageDir(tools.Cfg.Paths.Static),
		apiv2.WithTUMOnline(tum.Catalog{Dao: dao.NewDaoWrapper()}),
		// Uploads land where v1 puts them, and section thumbnails go the way v1's do.
		apiv2.WithMassStorage(tools.Cfg.Paths.Mass),
		apiv2.WithVODStorage(tools.Cfg.Paths.Vod),
		apiv2.WithSectionImages(sectionimages.SectionImages{Manager: manager}),
		apiv2.WithPostProcessing(postprocessing.PostProcessing{Manager: manager}),
		apiv2.WithUploads(uploads.Uploads{Manager: manager}),
		apiv2.WithRealtimeHub(hub),
		// Ending a stream goes through the manager, which tells the viewers it is over.
		apiv2.WithStreamEnder(manager),
		apiv2.WithSelfStreamStarter(manager),
		apiv2.WithSubtitleSearch(tools.NewMeiliSearchFunctions()),
		// The Meilisearch filters and post-filtering in pkg/search decide who may find what.
		apiv2.WithSearch(search.SearchCatalog{Dao: dao.NewDaoWrapper(), Meili: tools.NewMeiliSearchFunctions()}),
	}
	if subtitles != nil {
		api2Opts = append(api2Opts, apiv2.WithSubtitleGenerator(subtitles, tools.Cfg.VoiceService.AuthToken))
	}
	api2Client := apiv2.New(dao.DB, api2Opts...)

	g, _ := errgroup.WithContext(ctx)

	g.Go(func() error {
		return api2Client.Run(grpcl)
	})

	g.Go(func() error {
		return http.Serve(httpl, newRouter(api2Client))
	})

	g.Go(func() error {
		return m.Serve()
	})

	// Metrics get their own listener: the scrape target stays inside the cluster,
	// unlike everything served on the web port.
	if port := tools.Cfg.MetricsPort; port != "" {
		metricsMux := http.NewServeMux()
		metricsMux.Handle("/metrics", apiv2.MetricsHandler())
		// Registered explicitly rather than by blank import, which would only reach
		// http.DefaultServeMux. pprof.Index serves the goroutine/heap/allocs sub-paths.
		metricsMux.HandleFunc("/debug/pprof/", pprof.Index)
		metricsMux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
		metricsMux.HandleFunc("/debug/pprof/profile", pprof.Profile)
		metricsMux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
		metricsMux.HandleFunc("/debug/pprof/trace", pprof.Trace)
		metrics := &http.Server{
			Addr:              ":" + port,
			Handler:           metricsMux,
			ReadHeaderTimeout: 5 * time.Second,
		}
		g.Go(func() error {
			slog.Info("serving metrics", "port", port)
			if err := metrics.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				return err
			}
			return nil
		})
		g.Go(func() error {
			<-ctx.Done()
			return metrics.Shutdown(context.WithoutCancel(ctx))
		})
	}

	if err = g.Wait(); err != nil && ctx.Err() != nil {
		// webserver gracefully shut down
		return nil
	}
	return err
}

func initCron(logger *slog.Logger, m *runner_manager.Manager, hub realtimehub.Hub) {
	daoWrapper := dao.NewDaoWrapper()
	tools.InitCronService()
	// Fetch students every 12 hours
	_ = tools.Cron.AddFunc("fetchCourses", tum.FetchCourses(daoWrapper), "0 */12 * * *")
	// Collect livestream stats (viewers) every minute
	_ = tools.Cron.AddFunc("collectStats", livestate.CollectStats(daoWrapper, hub), "0-59 * * * *")
	_ = tools.Cron.AddFunc("triggerDueStreamsRunner", func() {
		err := m.TriggerDueStreams()
		if err != nil {
			logger.With("err", err).Error("Can't run streams with runner")
		}
	}, "0-59 * * * *")
	// update courses available
	_ = tools.Cron.AddFunc("prefetchCourses", tum.PrefetchCourses(daoWrapper), "30 3 * * *")
	// export data to meili search
	_ = tools.Cron.AddFunc("exportToMeili", tools.NewMeiliExporter(daoWrapper).Export, "30 4 * * *")
	// fetch live stream previews
	_ = tools.Cron.AddFunc("fetchLivePreviews", livepreview.Fetch(daoWrapper, m), "*/1 * * * *")
	// reap streams stuck in live state due to runner crash or lost notifications
	_ = tools.Cron.AddFunc("reapStaleStreams", m.ReapStaleStreams, "1/30 * * * *")
	// apply correct live lights
	_ = tools.Cron.AddFunc("updateLights", m.UpdateLights, "1/5 * * * *")

	tools.Cron.Run()
}
