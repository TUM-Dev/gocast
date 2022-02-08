package main

import (
	"TUM-Live/api"
	"TUM-Live/dao"
	"TUM-Live/model"
	"TUM-Live/tools"
	"TUM-Live/tools/tum"
	"TUM-Live/web"
	"fmt"
	"github.com/dgraph-io/ristretto"
	"github.com/droundy/goopt"
	"github.com/getsentry/sentry-go"
	sentrygin "github.com/getsentry/sentry-go/gin"
	"github.com/gin-contrib/gzip"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
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

// GinServer launch gin server
func GinServer() (err error) {
	router := gin.Default()
	// capture performance with sentry
	router.Use(sentrygin.New(sentrygin.Options{Repanic: true}))
	store := cookie.NewStore([]byte(tools.Cfg.CookieStoreSecret))
	if VersionTag != "development" {
		store.Options(sessions.Options{
			HttpOnly: true,
			Secure:   true,
			SameSite: http.SameSiteStrictMode,
			MaxAge:   86400 * 30,
		})
	}
	router.Use(sessions.Sessions("TUMLiveSessionV6", store))

	router.Use(tools.InitContext)

	// event streams don't work with gzip, configure group without
	chat := router.Group("/api/chat")
	api.ConfigChatRouter(chat)

	router.Use(gzip.Gzip(gzip.DefaultCompression))
	api.ConfigGinRouter(router)
	web.ConfigGinRouter(router)
	err = router.Run(":8081")
	if err != nil {
		sentry.CaptureException(err)
		log.WithError(err).Fatal("Error starting server")
	}
	return
}

var (
	OsSignal chan os.Signal
)

func main() {
	defer profile.Start(profile.MemProfile).Stop()
	go func() {
		_ = http.ListenAndServe(":8082", nil) // debug endpoint
	}()

	// log with time, fmt "23.09.2021 10:00:00"
	log.SetFormatter(&log.TextFormatter{TimestampFormat: "02.01.2006 15:04:05", FullTimestamp: true})

	web.VersionTag = VersionTag
	OsSignal = make(chan os.Signal, 1)

	goopt.Parse(nil)
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
		"%v:%v@tcp(db:3306)/%v?parseTime=true&loc=Local",
		tools.Cfg.Db.User,
		tools.Cfg.Db.Password,
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
		err := GinServer()
		if err != nil {
			sentry.CaptureException(err)
			sentry.Flush(time.Second * 5)
			log.WithError(err).Fatal("can't launch gin server")
		}
	}()
	LoopForever()
}

func initCron() {
	cronService := cron.New()
	//Fetch students every 12 hours
	_, _ = cronService.AddFunc("0 */12 * * *", tum.FetchCourses)
	//Collect livestream stats (viewers) every minute
	_, _ = cronService.AddFunc("0-59 * * * *", api.CollectStats)
	//Flush stale sentry exceptions and transactions every 5 minutes
	_, _ = cronService.AddFunc("0-59/5 * * * *", func() { sentry.Flush(time.Minute * 2) })
	//Look for due streams and notify workers about them
	_, _ = cronService.AddFunc("0-59 * * * *", api.NotifyWorkers)
	cronService.Start()
}

// LoopForever on signal processing
func LoopForever() {
	log.Info("Entering infinite loop\n")

	signal.Notify(OsSignal, syscall.SIGINT, syscall.SIGTERM, syscall.SIGUSR1)
	<-OsSignal
	log.Info("Exiting infinite loop received OsSignal\n")
}
