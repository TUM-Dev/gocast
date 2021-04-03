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
	"github.com/gin-contrib/gzip"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/robfig/cron/v3"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"math/rand"

	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
)

const UserKey = "RBG-Default-User" // UserKey key used for storing User struct in context

// GinServer launch gin server
func GinServer() (err error) {
	router := gin.Default()
	secret := make([]byte, 40) // 40 random bytes as cookie secret
	_, err = rand.Read(secret)
	if err != nil {
		log.Fatalf("Unable to generate cookie store secret: %err\n", err)
	}
	store := cookie.NewStore(secret)
	router.Use(sessions.Sessions("TUMLiveSessionV3", store))
	// event streams don't work with gzip, configure group without
	chat := router.Group("/api/chat")
	api.ConfigChatRouter(chat)

	router.Use(gzip.Gzip(gzip.DefaultCompression))
	api.ConfigGinRouter(router)
	web.ConfigGinRouter(router)
	err = router.Run(":8080")
	if err != nil {
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
	OsSignal = make(chan os.Signal, 1)

	goopt.Parse(nil)

	db, err := gorm.Open(mysql.Open(fmt.Sprintf(
		"%v:%v@tcp(db:3306)/%v?parseTime=true&loc=Local",
		tools.Cfg.DatabaseUser,
		tools.Cfg.DatabasePassword,
		tools.Cfg.DatabaseName),
	), &gorm.Config{})
	if err != nil {
		log.Fatalf("Got error when connecting to database: '%v'", err)
	}

	err = db.AutoMigrate(
		&model.User{},
		&model.Student{},
		&model.Course{},
		&model.Chat{},
		&model.RegisterLink{},
		&model.Stat{},
		&model.Stream{},
		&model.ProcessingJob{},
		&model.Worker{},
	)
	if err != nil {
		log.Fatalf("Could not migrate database: %v", err)
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

	cache, err := ristretto.NewCache(&ristretto.Config{
		NumCounters: 1e7,     // number of keys to track frequency of (10M).
		MaxCost:     1 << 30, // maximum cost of cache (1GB).
		BufferItems: 64,      // number of keys per Get buffer.
	})
	if err != nil {
		panic(err)
	}
	dao.Cache = *cache

	cronService := cron.New()
	//Fetch students every 12 hours
	_, _ = cronService.AddFunc("0 */12 * * *", tum.FetchCourses)
	_, _ = cronService.AddFunc("0-59/5 * * * *", api.CollectStats)
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
