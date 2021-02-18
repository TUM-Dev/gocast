package main

import (
	"TUM-Live-Backend/api"
	"TUM-Live-Backend/dao"
	"TUM-Live-Backend/model"
	"context"
	"fmt"
	"github.com/droundy/goopt"
	"github.com/gin-gonic/gin"
	//_ "github.com/golang-migrate/migrate/v4/database/postgres"
	//_ "github.com/golang-migrate/migrate/v4/source/github"
	"github.com/jmoiron/sqlx"
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

	api.ConfigGinRouter(router)
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

// @title Sample CRUD api for rbglive db
// @version 1.0
// @description Sample CRUD api for rbglive db
// @termsOfService
// @host localhost:8080
// @BasePath /
func main() {
	OsSignal = make(chan os.Signal, 1)

	goopt.Parse(nil)

	db, err := sqlx.Open("postgres", "host=db port=5432 user=postgres database=rbglive password=changeme sslmode=disable")
	if err != nil {
		log.Fatalf("Got error when connecting to database: '%v'", err)
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
	//todo fix database
	/* todo: auto migrate database
	// Migrate database
	driver, err := postgres.WithInstance(db.DB, &postgres.Config{})
	if err != nil {
		log.Fatal("couldn't migrate database")
	}
	m, err := migrate.NewWithDatabaseInstance(
		"file://migrations",
		"rbgreater", driver)
	if err != nil {
		log.Fatal("couldn't migrate database")
	}
	_ = m.Steps(2)
	*/

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

	api.RequestValidator = func(ctx context.Context, r *http.Request, table string, action model.Action) error {
		user, ok := UserFromContext(ctx)
		if !ok {
			return fmt.Errorf("unknown user")
		}

		fmt.Printf("user: %v accessing %s action: %v\n", user, table, action)
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
