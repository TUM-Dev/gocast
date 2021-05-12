package main

import (
	"TUM-Live/api"
	"TUM-Live/dao"
	"TUM-Live/model"
	"TUM-Live/tools"
	"TUM-Live/tools/tum"
	"TUM-Live/web"
	"context"
	"fmt"
	"github.com/dgraph-io/ristretto"
	"github.com/droundy/goopt"
	"github.com/getsentry/sentry-go"
	sentrygin "github.com/getsentry/sentry-go/gin"
	"github.com/gin-contrib/gzip"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/robfig/cron/v3"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

var VersionTag = "development"

const UserKey = "RBG-Default-User" // UserKey key used for storing User struct in context

// GinServer launch gin server
func GinServer() (err error) {
	router := gin.Default()
	// capture performance with sentry
	router.Use(sentrygin.New(sentrygin.Options{Repanic: true}))
	store := cookie.NewStore([]byte(tools.Cfg.CookieStoreSecret))
	router.Use(sessions.Sessions("TUMLiveSessionV4", store))

	// event streams don't work with gzip, configure group without
	chat := router.Group("/api/chat")
	api.ConfigChatRouter(chat)

	router.Use(gzip.Gzip(gzip.DefaultCompression))
	api.ConfigGinRouter(router)
	web.ConfigGinRouter(router)
	err = router.Run(":8081")
	if err != nil {
		sentry.CaptureException(err)
		log.Fatalf("Error starting server, the error is '%v'", err)
	}
	return
}

var (
	OsSignal chan os.Signal
)

// User struct to store database related info in context
type User struct {
	Name string
}

func (u *User) String() string {
	return u.Name
}

func main() {
	web.VersionTag = VersionTag
	OsSignal = make(chan os.Signal, 1)

	goopt.Parse(nil)
	if os.Getenv("SentryDSN") == "" {
		panic("need sentry dsn to run.")
	}
	err := sentry.Init(sentry.ClientOptions{
		Dsn:              os.Getenv("SentryDSN"),
		Release:          VersionTag,
		TracesSampleRate: 0.1,
		Debug:            true,
		AttachStacktrace: true,
	})
	if err != nil {
		log.Fatalf("sentry.Init: %s", err)
	}
	// Flush buffered events before the program terminates.
	defer sentry.Flush(2 * time.Second)
	defer sentry.Recover()

	db, err := gorm.Open(mysql.Open(fmt.Sprintf(
		"%v:%v@tcp(db:3306)/%v?parseTime=true&loc=Local",
		tools.Cfg.DatabaseUser,
		tools.Cfg.DatabasePassword,
		tools.Cfg.DatabaseName),
	), &gorm.Config{})
	if err != nil {
		sentry.CaptureException(err)
		sentry.Flush(time.Second * 5)
		log.Fatalf("%v", err)
	}

	err = db.AutoMigrate(
		&model.User{},
		&model.Student{},
		&model.Course{},
		&model.Chat{},
		&model.RegisterLink{},
		&model.Stat{},
		&model.StreamUnit{},
		&model.LectureHall{},
		&model.Stream{},
		&model.ProcessingJob{},
		&model.Worker{},
		&model.CameraPreset{},
	)
	if err != nil {
		sentry.CaptureException(err)
		sentry.Flush(time.Second * 5)
		log.Fatalf("%v", err)
	}

	dao.DB = db
	dao.Logger = func(ctx context.Context, sql string) {
		user, ok := UserFromContext(ctx)
		if ok {
			fmt.Printf("[%v] SQL: %s\n", user, sql)
		} else {
			fmt.Printf("SQL: %s\n", sql)
		}
	}

	//tools.FetchCameraPresets()

	cache, err := ristretto.NewCache(&ristretto.Config{
		NumCounters: 1e7,     // number of keys to track frequency of (10M).
		MaxCost:     1 << 30, // maximum cost of cache (1GB).
		BufferItems: 64,      // number of keys per Get buffer.
	})
	if err != nil {
		sentry.CaptureException(err)
		sentry.Flush(time.Second * 5)
		log.Fatalf("%v", err)
	}
	dao.Cache = *cache

	cronService := cron.New()
	//Fetch students every 12 hours
	_, _ = cronService.AddFunc("0 */12 * * *", tum.FetchCourses)
	_, _ = cronService.AddFunc("0-59 * * * *", api.CollectStats)
	_, _ = cronService.AddFunc("0-59/5 * * * *", func() { sentry.Flush(time.Minute * 2) })
	_, _ = cronService.AddFunc("0-59 * * * *", tools.NotifyWorkers)
	cronService.Start()
	api.ContextInitializer = func(r *http.Request) (ctx context.Context) {
		val, ok := r.Header["X-Api-User"]
		if ok {
			if len(val) > 0 {
				u := &User{Name: val[0]}
				ctx = r.Context()
				ctx = context.WithValue(ctx, UserKey, u)
				r.WithContext(ctx)
			}
		}

		if ctx == nil {
			ctx = r.Context()
		}

		return ctx
	}

	api.RequestValidator = func(ctx context.Context, r *http.Request, table string) error {
		user, ok := UserFromContext(ctx)
		if !ok {
			return fmt.Errorf("unknown user")
		}

		fmt.Printf("user: %v accessing %s ", user, table)
		return nil
	}
	go GinServer()
	LoopForever()
}

// UserFromContext retrieve a User from Context if available
func UserFromContext(ctx context.Context) (*User, bool) {
	u, ok := ctx.Value(UserKey).(*User)
	return u, ok
}

// LoopForever on signal processing
func LoopForever() {
	fmt.Printf("Entering infinite loop\n")

	signal.Notify(OsSignal, syscall.SIGINT, syscall.SIGTERM, syscall.SIGUSR1)
	_ = <-OsSignal

	fmt.Printf("Exiting infinite loop received OsSignal\n")
}
