// Command seeduser creates (or resets) the account the end-to-end suite signs in
// with, so a run does not depend on whatever happens to be in the local database.
//
//	go run ./frontend/e2e/seeduser
//
// It reads the same config.yaml the server reads for its database credentials, but
// unlike tools.LoadConfig it never writes the file back. Credentials come from
// E2E_USERNAME and E2E_PASSWORD when set, matching what playwright.config.ts passes
// to the tests; the defaults below are what both fall back to.
//
// This writes to the database, so point it at a development instance only. Nothing
// runs it automatically.
package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"

	"github.com/spf13/viper"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/TUM-Dev/gocast/model"
)

const (
	defaultUsername = "e2e@localhost"
	defaultPassword = "e2e-password"
)

type dbConfig struct {
	Host     string
	Port     int
	Database string
	User     string
	Password string
}

func main() {
	// The login form's "username" field is matched against the email column
	// (loginWithUserCredentials in web/user.go), so that is what gets seeded.
	email := envOr("E2E_USERNAME", defaultUsername)
	password := envOr("E2E_PASSWORD", defaultPassword)

	cfg, err := readDBConfig()
	if err != nil {
		log.Fatalf("reading config: %v", err)
	}

	db, err := gorm.Open(mysql.Open(fmt.Sprintf(
		"%s:%s@tcp(%s:%d)/%s?parseTime=true&loc=Local",
		cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.Database,
	)), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		log.Fatalf("connecting to %s:%d: %v", cfg.Host, cfg.Port, err)
	}

	var user model.User
	// Find rather than First: a missing row is the normal case on a fresh database,
	// and Find leaves the struct zeroed instead of returning an error.
	if err := db.Where("email = ?", email).Limit(1).Find(&user).Error; err != nil {
		log.Fatalf("looking up %s: %v", email, err)
	}
	existed := user.ID != 0

	if user.Name == "" {
		user.Name = "End To End"
	}
	user.Email = sql.NullString{String: email, Valid: true}
	// A student can reach /settings and change every setting the page offers, which is
	// all the suite needs; a higher role would also hide missing-permission bugs.
	user.Role = model.StudentType
	if err := user.SetPassword(password); err != nil {
		log.Fatalf("hashing password: %v", err)
	}

	if err := db.Save(&user).Error; err != nil {
		log.Fatalf("saving %s: %v", email, err)
	}

	action := "created"
	if existed {
		action = "updated"
	}
	log.Printf("%s user %q (id %d) with the end-to-end password", action, email, user.ID)
}

func readDBConfig() (dbConfig, error) {
	v := viper.New()
	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath("/etc/TUM-Live/")
	v.AddConfigPath("$HOME/.TUM-Live")
	v.AddConfigPath(".")
	v.AddConfigPath("..")
	v.AddConfigPath("../..")
	if err := v.ReadInConfig(); err != nil {
		return dbConfig{}, err
	}

	var cfg dbConfig
	return cfg, v.UnmarshalKey("db", &cfg)
}

func envOr(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
