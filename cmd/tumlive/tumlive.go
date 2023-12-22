package main

import (
	"fmt"
	"log/slog"
	"net"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/TUM-Dev/gocast/api"
	"github.com/TUM-Dev/gocast/api_v2"
	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/model"
	"github.com/TUM-Dev/gocast/tools"
	"github.com/TUM-Dev/gocast/tools/tum"
	"github.com/TUM-Dev/gocast/web"
	"github.com/dgraph-io/ristretto"
	"github.com/getsentry/sentry-go"
	sentrygin "github.com/getsentry/sentry-go/gin"
	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
	slogGorm "github.com/orandin/slog-gorm"
	"github.com/pkg/profile"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

var VersionTag = "development"

type initializer func()

var logger = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
	Level: slog.LevelDebug,
})).With("service", "main")

var initializers = []initializer{
	tools.LoadConfig,
	api.ServeWorkerGRPC,
	tools.InitBranding,
}

func initAll(initializers []initializer) {
	for _, init := range initializers {
		init()
	}
}

// GinServer launches the gin server
func GinServer() (err error) {
	router := gin.New()
	router.Use(gin.Recovery())
	gin.SetMode(gin.ReleaseMode)
	// capture performance with sentry
	router.Use(sentrygin.New(sentrygin.Options{Repanic: true}))
	if VersionTag != "development" {
		tools.CookieSecure = true
	}

	router.Use(gin.LoggerWithFormatter(func(param gin.LogFormatterParams) string {
		return fmt.Sprintf("{\"service\": \"GIN\", \"time\": %s, \"status\": %d, \"client\": \"%s\", \"path\": \"%s\", \"agent\": %s}\n",
			param.TimeStamp.Format(time.DateTime),
			param.StatusCode,
			param.ClientIP,
			param.Path,
			param.Request.UserAgent(),
		)
	}))

	router.Use(tools.InitContext(dao.NewDaoWrapper()))

	l, err := net.Listen("tcp", ":8081")
	if err != nil {
		logger.Error("can't listen on port 8081", "err", err)
	}

	api2Client := api_v2.New(dao.DB)
	go func() {
		if err := api2Client.Run(l); err != nil {
			logger.Error("can't launch grpc server", "err", err)
		}
	}()

	liveUpdates := router.Group("/api/pub-sub")
	api.ConfigRealtimeRouter(liveUpdates)

	// event streams don't work with gzip, configure group without
	chat := router.Group("/api/chat")
	api.ConfigChatRouter(chat)

	router.Use(gzip.Gzip(gzip.DefaultCompression))
	router.Any("/api/v2/*any", api2Client.Proxy())
	api.ConfigGinRouter(router)
	web.ConfigGinRouter(router)
	err = router.RunListener(l)
	// err = router.RunTLS(":443", tools.Cfg.Saml.Cert, tools.Cfg.Saml.Privkey)
	if err != nil {
		sentry.CaptureException(err)
		logger.Error("Error starting tumlive", "err", err)
	}
	return
}

var osSignal chan os.Signal

func main() {
	initAll(initializers)

	defer profile.Start(profile.MemProfile).Stop()
	go func() {
		_ = http.ListenAndServe(":8082", nil) // debug endpoint
	}()

	// log with time, fmt "23.09.2021 10:00:00"
	// log.SetFormatter(&log.TextFormatter{TimestampFormat: "02.01.2006 15:04:05", FullTimestamp: true})

	web.VersionTag = VersionTag
	osSignal = make(chan os.Signal, 1)

	env := "production"
	if VersionTag == "development" {
		env = "development"
	}
	if os.Getenv("SentryDSN") != "" {
		err := sentry.Init(sentry.ClientOptions{
			Dsn:              os.Getenv("SentryDSN"),
			Release:          VersionTag,
			TracesSampleRate: 0.15,
			Debug:            true,
			AttachStacktrace: true,
			Environment:      env,
		})
		if err != nil {
			logger.Error("sentry.Init", "err", err)
		}
		// Flush buffered events before the program terminates.
		defer sentry.Flush(2 * time.Second)
		defer sentry.Recover()
	}

	gormJSONLogger := slogGorm.New()

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
		sentry.CaptureException(err)
		sentry.Flush(time.Second * 5)
		logger.Error("Error opening database", "err", err)
	}
	dao.DB = db

	err = dao.Migrator.RunBefore(db)
	if err != nil {
		logger.Error("Error running before db", "err", err)
		return
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
		&model.Device{},
	)
	if err != nil {
		sentry.CaptureException(err)
		sentry.Flush(time.Second * 5)
		logger.Error("can't migrate database", "err", err)
	}
	err = dao.Migrator.RunAfter(db)
	if err != nil {
		logger.Error("Error running after db", "err", err)
		return
	}

	// tools.SwitchPreset()

	cache, err := ristretto.NewCache(&ristretto.Config{
		NumCounters: 1e7,     // number of keys to track frequency of (10M).
		MaxCost:     1 << 30, // maximum cost of cache (1GB).
		BufferItems: 64,      // number of keys per Get buffer.
		Metrics:     true,
	})
	if err != nil {
		sentry.CaptureException(err)
		sentry.Flush(time.Second * 5)
		logger.Error("Error risretto.NewCache", "err", err)
	}
	dao.Cache = *cache

	// init meili search index settings
	go tools.NewMeiliExporter(dao.NewDaoWrapper()).SetIndexSettings()

	mailer := tools.NewMailer(dao.NewDaoWrapper(), tools.Cfg.Mail.MaxMailsPerMinute)
	go mailer.Run()

	initCron()
	go func() {
		err = GinServer()
		if err != nil {
			sentry.CaptureException(err)
			sentry.Flush(time.Second * 5)
			logger.Error("can't launch gin server", "err", err)
		}
	}()
	keepAlive()
}

func initCron() {
	daoWrapper := dao.NewDaoWrapper()
	tools.InitCronService()
	// Fetch students every 12 hours
	_ = tools.Cron.AddFunc("fetchCourses", tum.FetchCourses(daoWrapper), "0 */12 * * *")
	// Collect livestream stats (viewers) every minute
	_ = tools.Cron.AddFunc("collectStats", api.CollectStats(daoWrapper), "0-59 * * * *")
	// Flush stale sentry exceptions and transactions every 5 minutes
	_ = tools.Cron.AddFunc("sentryFlush", func() { sentry.Flush(time.Minute * 2) }, "0-59/5 * * * *")
	// Look for due streams and notify workers about them
	_ = tools.Cron.AddFunc("triggerDueStreams", api.NotifyWorkers(daoWrapper), "0-59 * * * *")
	// update courses available
	_ = tools.Cron.AddFunc("prefetchCourses", tum.PrefetchCourses(daoWrapper), "30 3 * * *")
	// export data to meili search
	_ = tools.Cron.AddFunc("exportToMeili", tools.NewMeiliExporter(daoWrapper).Export, "30 4 * * *")
	// fetch live stream previews
	_ = tools.Cron.AddFunc("fetchLivePreviews", api.FetchLivePreviews(daoWrapper), "*/1 * * * *")
	tools.Cron.Run()
}

func keepAlive() {
	signal.Notify(osSignal, syscall.SIGINT, syscall.SIGTERM, syscall.SIGUSR1)
	s := <-osSignal
	logger.Info("Exiting on signal" + s.String())
}
