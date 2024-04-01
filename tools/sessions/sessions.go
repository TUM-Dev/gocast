package sessions

import (
	"database/sql"
	"github.com/SebiWrn/gin-sessions"
	"github.com/SebiWrn/gin-sessions/stores/mysqlstore"
	"github.com/TUM-Dev/gocast/tools"
	"strconv"

	"log/slog"
	"os"
)

import _ "github.com/go-sql-driver/mysql"

var Store sessions.Store

var dsn string

func InitSessions() {
	dsn = tools.Cfg.Db.User + ":" + tools.Cfg.Db.Password + "@tcp(" + tools.Cfg.Db.Host + ":" + strconv.Itoa(int(tools.Cfg.Db.Port)) + ")/" + tools.Cfg.Db.SessionDB + "?parseTime=true&loc=Local"

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		slog.Error("Error opening database", "err", err)
		os.Exit(1)
	}

	Store, err = mysqlstore.NewMySQLStoreFromConnection(db, "sessions", "/", 3600, []byte(os.Getenv("STORE_SECRET")))
	if err != nil {
		slog.Error("Error creating session store", "err", err)
		os.Exit(1)
	}

	slog.Info("Successfully established connection to session store")
}
