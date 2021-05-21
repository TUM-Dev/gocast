package api

import (
	"context"
	"github.com/gin-gonic/gin"
	_ "github.com/satori/go.uuid"
	"net/http"
	"time"
)

var (
	_ = time.Second // import time.Second for unknown usage in api
)

// PagedResults results for pages GetAll results.
type PagedResults struct {
	Page         int64       `json:"Page"`
	PageSize     int64       `json:"PageSize"`
	Data         interface{} `json:"Data"`
	TotalRecords int         `json:"TotalRecords"`
}

// HTTPError example
type HTTPError struct {
	Code    int    `json:"Code" example:"400"`
	Message string `json:"Message" example:"status bad request"`
}

// ConfigGinRouter configure gin router
func ConfigChatRouter(router gin.IRoutes) {
	configGinChatRouter(router)
}
func ConfigGinRouter(router gin.IRoutes) {
	configGinStreamAuthRouter(router)
	configGinUsersRouter(router)
	configGinCourseRouter(router)
	configGinWorkerRouter(router)
	configGinDownloadRouter(router)
	configGinLectureHallApiRouter(router)
	configGinSexyApiRouter(router)
	return
}

type RequestValidatorFunc func(ctx context.Context, r *http.Request, table string) error

var RequestValidator RequestValidatorFunc

type ContextInitializerFunc func(r *http.Request) (ctx context.Context)

var ContextInitializer ContextInitializerFunc
