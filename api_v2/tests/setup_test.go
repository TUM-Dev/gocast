package tests

import (
	"fmt"
	"log/slog"
	"net"
	_ "net/http/pprof"
	"os"
	"testing"
	"time"

	"github.com/TUM-Dev/gocast/api_v2"
	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/model"
	"github.com/TUM-Dev/gocast/tools"
	"github.com/getsentry/sentry-go"
	"github.com/golang-jwt/jwt/v4"
	slogGorm "github.com/orandin/slog-gorm"
	"google.golang.org/grpc/metadata"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

type initializer func()

var logger = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
	Level: slog.LevelDebug,
})).With("service", "main")

var initializers = []initializer{
	tools.LoadConfig,
	tools.InitBranding,
}

var (
	a                   *api_v2.API
	md_student_loggedin metadata.MD // LoggedIn user which is not enrolled in any courses
	md_student_enrolled metadata.MD // LoggedIn user which is not enrolled in courses
	md_admin            metadata.MD
	md_invalid_jwt      metadata.MD
)

func createToken(user uint) (string, error) {
	t := jwt.New(jwt.GetSigningMethod("RS256"))

	t.Claims = &tools.JWTClaims{
		RegisteredClaims: &jwt.RegisteredClaims{
			ExpiresAt: &jwt.NumericDate{Time: time.Now().Add(time.Hour * 24 * 7)}, // Token expires in one week
		},
		UserID: user,
	}
	return t.SignedString(tools.Cfg.GetJWTKey())
}

func setupJWTs() {
	jwt_student_loggedin, _ := createToken(7)
	md_student_loggedin = metadata.New(map[string]string{
		"grpcgateway-cookie": "jwt=" + jwt_student_loggedin,
	})

	jwt_student_enrolled, _ := createToken(8)
	md_student_enrolled = metadata.New(map[string]string{
		"grpcgateway-cookie": "jwt=" + jwt_student_enrolled,
	})

	jwt_admin, _ := createToken(1)
	md_admin = metadata.New(map[string]string{
		"grpcgateway-cookie": "jwt=" + jwt_admin,
	})

	md_invalid_jwt = metadata.New(map[string]string{
		"grpcgateway-cookie": "jwt=eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.eyJleI6MSwiU2FtbHAiOjE3MDQ4MDcyMzUsIlVzZXJJRCFN1YmplY3RJRCI6bnVsbH0.a400epuWDTjYUEf_5wCA2SwE-l0klXZ3B6n98qhy4_oWJ0lX13TbZG5-iSFJrohxXEL-wtMyevJBFFQlWTJthgA6E8izqUd_zKml2i65ukzMX5M8Lf83nQfCj8WaqLXs3ocI3szIRJR9mBVd0d1VMGDl-xeFHzlSBbpaWqdNE8ND_NGKUamwy1Sx7UkLx4_NC0Ovr7i_xAgCcrZAFHVh6bcmod28ZPlmcW837yvTllwjsbvkaJDfK15R4heIX7iBdtYU3zfZwH5sebgb1gbd74TWKm0Sgn6Jg2WOpH-PcVPu9HlGJ8c09Ff_vFq7UpmHzBmNDQxai4tZ4F87beXdPw",
	})
}

func TestMain(m *testing.M) {
	// Call the setup function to get an instance of your API
	a = setup()

	// Set the JWTs
	setupJWTs()

	// Run the tests
	code := m.Run()

	// Exit with the code returned from running the tests
	os.Exit(code)
}

func initAll(initializers []initializer) {
	for _, init := range initializers {
		init()
	}
}

func setup() *api_v2.API {
	initAll(initializers)

	// Create a test instance of your API
	gormJSONLogger := slogGorm.New()
	db, err := gorm.Open(mysql.Open(fmt.Sprintf(
		"%s:%s@tcp(%s:%d)/%s?parseTime=true&loc=Local",
		tools.Cfg.Db.User,
		tools.Cfg.Db.Password,
		tools.Cfg.Db.Host,
		tools.Cfg.Db.Port,
		"tumlive_test"),
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
		return nil
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
		return nil
	}

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

	return api2Client
}
