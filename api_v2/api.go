package api_v2

//go:generate ./generate.sh

import (
	"context"
	"embed"
	_ "embed"
	"github.com/TUM-Dev/gocast/api_v2/protobuf"
	"github.com/gin-gonic/gin"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/reflection"
	"gorm.io/gorm"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strings"
	"time"
	"fmt"
	"github.com/NaySoftware/go-fcm"
)

// API is the grpc server for the v2 api
type API struct {
	db  *gorm.DB
	log *slog.Logger

	protobuf.UnimplementedAPIServer
}

// New creates a new API and assigns the given db and a logger
func New(db *gorm.DB) *API {
	log := slog.With("apiVersion", "2")
	return &API{
		db:  db,
		log: log,
	}
}

// Run starts the grpc server on port 12544 and the grpc gateway on ::8081/api/v2
func (a *API) Run(net.Listener) error {
	a.log.Info("Running")
	lis, err := net.Listen("tcp", ":12544")
	if err != nil {
		return err
	}
	grpcServer := grpc.NewServer(grpc.KeepaliveParams(keepalive.ServerParameters{
		MaxConnectionIdle:     time.Minute,
		MaxConnectionAge:      time.Minute,
		MaxConnectionAgeGrace: time.Second * 5,
		Time:                  time.Minute * 10,
		Timeout:               time.Second * 20,
	}))

	protobuf.RegisterAPIServer(grpcServer, a)
	reflection.Register(grpcServer)
	// TODO: Check with @Joscha
	// Send notification for testing purposes to simulate stream upload
	a.sendTestNotification()
	return grpcServer.Serve(lis)
}

// Proxy returns a gin handler that proxies requests to the grpc gateway server
func (a *API) Proxy() func(c *gin.Context) {
	// setup muxing
	mux := runtime.NewServeMux()
	opts := []grpc.DialOption{grpc.WithInsecure()}
	err := protobuf.RegisterAPIHandlerFromEndpoint(context.Background(), mux, ":12544", opts)
	if err != nil {
		a.log.With("err", err).Error("can't register grpc handler")
		os.Exit(1)
	}

	// actual proxy method forwards the request to the grpc gateway server
	return func(c *gin.Context) {
		if c.Request.Method == http.MethodGet && strings.HasPrefix(c.Request.URL.Path, "/api/v2/docs") {
			a.handleDocs(c)
			return
		}
		http.StripPrefix("/api/v2", mux).ServeHTTP(c.Writer, c.Request)
	}
}

//go:embed docs
var openApiJson embed.FS

// handleDocs serves the openapi.json file and the swagger ui
func (a *API) handleDocs(c *gin.Context) {
	httpFs := http.FS(openApiJson)
	fileServer := http.FileServer(httpFs)
	http.StripPrefix("/api/v2", fileServer).ServeHTTP(c.Writer, c.Request)
}

// Temporary method to trigger push notifications for given courseID, for actual implementation see ./api/courses.bo and ./dao/courses.go
func (a *API) sendTestNotification() {
	streamID := 7
	a.log.Info("Start finding device tokens for stream", streamID)
	var deviceTokens []string
    query := `
        SELECT devices.device_token
        FROM devices
        JOIN course_users ON devices.user_id = course_users.user_id
        JOIN streams ON course_users.course_id = streams.course_id
        WHERE streams.id = ?
	`
    err := a.db.Raw(query, streamID).Scan(&deviceTokens).Error
    if err != nil {
		a.log.Error("Error finding device tokens")
        return
    }

	a.log.Info(fmt.Sprintf("Start sending push notifications to devices: %d", len(deviceTokens)))
	// THIS IS ONLY A PLACEHOLDER - INSERT YOUR OWN SERVER KEY BEFORE STARTING THE API LOCALLY
	serverKey := "AAAA_8kwlHY:APA91bGiTBx4IhYg5xvHAHD7r4cI44IgUpkeNOMkftcnjyL_ayaqAedKOwzKhD53mT9GfFhX8XTNwRXIktNMrIgzLXAZnWBrOiHbCLE1rqr90SZ-STa3O3gzjJFNwlAfEgIyF7ln9_ku"

   
	data := map[string]string{
		"sum": "New VOD available!",
		"msg": "Stream name (Stream title)",
	}
 
	if err != nil {
		a.log.Error("Could not find subscribed users")
		return
	}

	fcm_c := fcm.NewFcmClient(serverKey)
	fcm_c.NewFcmRegIdsMsg(deviceTokens, data)
	status, err := fcm_c.Send()
	if err != nil {
		a.log.Error("Error sending push notifications")
		return
	}

	a.log.Info("Sent push notifications to devices: ", status)	
}