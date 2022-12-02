package main

import (
	"fmt"
	"github.com/dgraph-io/ristretto"
	"github.com/getsentry/sentry-go"
	sentrygin "github.com/getsentry/sentry-go/gin"
	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
	"github.com/joschahenningsen/TUM-Live/api"
	"github.com/joschahenningsen/TUM-Live/dao"
	"github.com/joschahenningsen/TUM-Live/model"
	"github.com/joschahenningsen/TUM-Live/tools"
	"github.com/joschahenningsen/TUM-Live/tools/tum"
	"github.com/joschahenningsen/TUM-Live/web"
	"github.com/pkg/profile"
	"github.com/robfig/cron/v3"
	log "github.com/sirupsen/logrus"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"syscall"
	"time"
)

var VersionTag = "development"

type initializer func()

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
	router := gin.Default()
	gin.SetMode(gin.ReleaseMode)
	// capture performance with sentry
	router.Use(sentrygin.New(sentrygin.Options{Repanic: true}))
	if VersionTag != "development" {
		tools.CookieSecure = true
	}

	router.Use(tools.InitContext(dao.NewDaoWrapper()))

	liveUpdates := router.Group("/api/pub-sub")
	api.ConfigRealtimeRouter(liveUpdates)

	// event streams don't work with gzip, configure group without
	chat := router.Group("/api/chat")
	api.ConfigChatRouter(chat)

	router.Use(gzip.Gzip(gzip.DefaultCompression))
	api.ConfigGinRouter(router)
	web.ConfigGinRouter(router)
	err = router.Run(":8081")
	//err = router.RunTLS(":443", tools.Cfg.Saml.Cert, tools.Cfg.Saml.Privkey)
	if err != nil {
		sentry.CaptureException(err)
		log.WithError(err).Fatal("Error starting tumlive")
	}
	return
}

var (
	osSignal chan os.Signal
)

func main() {
	initAll(initializers)

	defer profile.Start(profile.MemProfile).Stop()
	go func() {
		_ = http.ListenAndServe(":8082", nil) // debug endpoint
	}()

	// log with time, fmt "23.09.2021 10:00:00"
	log.SetFormatter(&log.TextFormatter{TimestampFormat: "02.01.2006 15:04:05", FullTimestamp: true})

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
			log.Fatalf("sentry.Init: %s", err)
		}
		// Flush buffered events before the program terminates.
		defer sentry.Flush(2 * time.Second)
		defer sentry.Recover()
	}
	db, err := gorm.Open(mysql.Open(fmt.Sprintf(
		"%s:%s@tcp(%s:%d)/%s?parseTime=true&loc=Local",
		tools.Cfg.Db.User,
		tools.Cfg.Db.Password,
		tools.Cfg.Db.Host,
		tools.Cfg.Db.Port,
		tools.Cfg.Db.Database),
	), &gorm.Config{
		PrepareStmt: true,
	})

	if err != nil {
		sentry.CaptureException(err)
		sentry.Flush(time.Second * 5)
		log.Fatalf("%v", err)
	}
	dao.DB = db

	err = dao.Migrator.RunBefore(db)
	if err != nil {
		log.Error(err)
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
		&model.Keyword{},
		&model.UserSetting{},
		&model.Audit{},
		&model.InfoPage{},
		&model.Bookmark{},
		&model.TranscodingProgress{},
		&model.PrefetchedCourse{},
	)
	if err != nil {
		sentry.CaptureException(err)
		sentry.Flush(time.Second * 5)
		log.WithError(err).Fatal("can't migrate database")
	}
	err = dao.Migrator.RunAfter(db)
	if err != nil {
		log.Error(err)
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
		log.Fatalf("%v", err)
	}
	dao.Cache = *cache
	initCron()
	go func() {
		err = GinServer()
		if err != nil {
			sentry.CaptureException(err)
			sentry.Flush(time.Second * 5)
			log.WithError(err).Fatal("can't launch gin server")
		}
	}()
	keepAlive()
}

func initCron() {
	daoWrapper := dao.NewDaoWrapper()
	cronService := cron.New()
	//Fetch students every 12 hours
	_, _ = cronService.AddFunc("0 */12 * * *", tum.FetchCourses(daoWrapper))
	//Collect livestream stats (viewers) every minute
	_, _ = cronService.AddFunc("0-59 * * * *", api.CollectStats(daoWrapper))
	//Flush stale sentry exceptions and transactions every 5 minutes
	_, _ = cronService.AddFunc("0-59/5 * * * *", func() { sentry.Flush(time.Minute * 2) })
	//Look for due streams and notify workers about them
	_, _ = cronService.AddFunc("0-59 * * * *", api.NotifyWorkers(daoWrapper))
	// update courses available every monday at 3am
	_, _ = cronService.AddFunc("30 3 * * *", tum.PrefetchCourses(daoWrapper))
	cronService.Start()
}

func keepAlive() {
	signal.Notify(osSignal, syscall.SIGINT, syscall.SIGTERM, syscall.SIGUSR1)
	s := <-osSignal
	log.Infof("Exiting on signal %s", s.String())
}
