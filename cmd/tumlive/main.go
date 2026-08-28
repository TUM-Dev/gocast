package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/dgraph-io/ristretto/v2"
	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
	slogGorm "github.com/orandin/slog-gorm"
	"github.com/soheilhy/cmux"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"github.com/TUM-Dev/gocast/api"
	apiv2 "github.com/TUM-Dev/gocast/apiv2/server"
	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/model"
	"github.com/TUM-Dev/gocast/pkg/camera"
	"github.com/TUM-Dev/gocast/pkg/runner_manager"
	"github.com/TUM-Dev/gocast/tools"
	"github.com/TUM-Dev/gocast/tools/tum"
	"github.com/TUM-Dev/gocast/voice-service/pb"
	"github.com/TUM-Dev/gocast/web"
)

func main() {
	ctx := context.Background()
	err := run(ctx)
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
	defer api.RealtimeInstance.CloseAll()

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
		&model.ShortLink{},
		&model.Stat{},
		&model.StreamUnit{},
		&model.LectureHall{},
		&model.IngestServer{},
		&model.StreamName{},
		&model.Stream{},
		&model.Worker{},
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
		&model.UploadKey{},
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

	opts := []runner_manager.Option{
		runner_manager.WithMassStorage(tools.Cfg.Paths.Mass),
		runner_manager.WithCamService(camService),
		runner_manager.WithLiveStateNotifier(api.NotifyViewersLiveState),
	}
	var subtitleClient pb.SubtitleGeneratorClient
	if tools.Cfg.VoiceService.Host != "" {
		api.RunVoiceServiceReceiver(tools.Cfg.VoiceService.AuthToken)
		c, err := grpc.NewClient(fmt.Sprintf("%s:%s", tools.Cfg.VoiceService.Host, tools.Cfg.VoiceService.Port), grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			logger.Error("failed to connect to voice service", "err", err)
		} else {
			opts = append(opts, runner_manager.WithSubtitleClient(pb.NewSubtitleGeneratorClient(c), tools.Cfg.VoiceService.AuthToken))
		}
	}
	m := runner_manager.New(dao.NewDaoWrapper(), opts...)
	logger.Info("running runner manager")
	err = m.Run()
	if err != nil {
		logger.Error("Failed to start runner manager", "err", err)
	}

	api.ServeWorkerGRPC(subtitleClient, tools.Cfg.VoiceService.AuthToken)

	// init meili search index settings
	go tools.NewMeiliExporter(dao.NewDaoWrapper()).SetIndexSettings()

	mailer := tools.NewMailer(dao.NewDaoWrapper(), tools.Cfg.Mail.MaxMailsPerMinute)
	go mailer.Run()

	initCron(logger, m)
	return serveHttp(ctx, m, camService)
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

// serveHttp launches all http servers
func serveHttp(ctx context.Context, manager *runner_manager.Manager, camService *camera.Service) (err error) {
	router := gin.New()
	router.Use(gin.Recovery())
	gin.SetMode(gin.ReleaseMode)
	if VersionTag != "development" {
		tools.CookieSecure = true
	}

	router.Use(gin.LoggerWithFormatter(func(param gin.LogFormatterParams) string {
		if param.StatusCode >= 400 && VersionTag == "development" {
			return fmt.Sprintf("{\"service\": \"GIN\", \"time\": %s, \"status\": %d, \"client\": \"%s\", \"path\": \"%s\", \"agent\": %s}\n",
				param.TimeStamp.Format(time.DateTime),
				param.StatusCode,
				param.ClientIP,
				param.Path,
				param.Request.UserAgent(),
			)
		}
		return ""
	}))

	router.Use(tools.InitContext(dao.NewDaoWrapper()))

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

	api2Client := apiv2.New(dao.DB)

	g, _ := errgroup.WithContext(ctx)

	g.Go(func() error {
		return api2Client.Run(grpcl)
	})

	liveUpdates := router.Group("/api/pub-sub")
	api.ConfigRealtimeRouter(liveUpdates)

	// event streams don't work with gzip, configure group without
	chat := router.Group("/api/chat")
	api.ConfigChatRouter(chat)

	router.Use(gzip.Gzip(gzip.DefaultCompression))
	router.Any("/api/v2/*any", api2Client.Proxy())
	api.ConfigGinRouter(router, manager, camService)
	web.ConfigGinRouter(router)
	g.Go(func() error {
		return router.RunListener(httpl)
	})

	g.Go(func() error {
		return m.Serve()
	})

	// Metrics get their own listener: the scrape target stays inside the cluster,
	// unlike everything served on the web port.
	if port := tools.Cfg.MetricsPort; port != "" {
		metricsMux := http.NewServeMux()
		metricsMux.Handle("/metrics", apiv2.MetricsHandler())
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

func initCron(logger *slog.Logger, m *runner_manager.Manager) {
	daoWrapper := dao.NewDaoWrapper()
	tools.InitCronService()
	// Fetch students every 12 hours
	_ = tools.Cron.AddFunc("fetchCourses", tum.FetchCourses(daoWrapper), "0 */12 * * *")
	// Collect livestream stats (viewers) every minute
	_ = tools.Cron.AddFunc("collectStats", api.CollectStats(daoWrapper), "0-59 * * * *")
	// Look for due streams and notify workers about them
	_ = tools.Cron.AddFunc("triggerDueStreams", api.NotifyWorkers(daoWrapper), "0-59 * * * *")
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
	_ = tools.Cron.AddFunc("fetchLivePreviews", api.FetchLivePreviews(daoWrapper), "*/1 * * * *")
	// reap streams stuck in live state due to runner crash or lost notifications
	_ = tools.Cron.AddFunc("reapStaleStreams", m.ReapStaleStreams, "1/30 * * * *")
	// apply correct live lights
	_ = tools.Cron.AddFunc("updateLights", m.UpdateLights, "1/5 * * * *")

	tools.Cron.Run()
}
