package main

import (
	"TUM-Live-Backend/api"
	"TUM-Live-Backend/dao"
	"TUM-Live-Backend/model"
	"TUM-Live-Backend/web"
	"context"
	"fmt"
	"github.com/droundy/goopt"
	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"

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

	db, err := gorm.Open(mysql.Open("root:example@tcp(db:3306)/rbglive?parseTime=true"), &gorm.Config{})
	if err != nil {
		log.Fatalf("Got error when connecting to database: '%v'", err)
	}

	err = db.AutoMigrate(
		&model.Course{},
		&model.CourseOwner{},
		&model.Session{},
		&model.Stream{},
		&model.User{},
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
