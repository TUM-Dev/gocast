// Package api is what is left of the v1 REST API. The SPA and every other first-party
// caller moved to apiv2 (Protobuf/Connect), and the shared code that used to live here
// (worker and voice gRPC, search, live state, section images) moved to pkg/. These
// routes remain because something that cannot speak apiv2 calls them:
//
//   - POST /api/selfstream/onPublish: the mediamtx ingest server's authHTTPAddress
//     webhook (ingest/mediamtx.yml), asked to approve every RTMP publish.
//   - POST /api/token/proxy/:token: rtmp-proxy/publish.sh, which swaps a lecturer's
//     token for the stream key of their next stream.
//   - GET /api/stream/live: API users authenticating with an admin token (scripts,
//     monitoring); the token middleware is not part of apiv2.
//   - GET /api/schedule.ics: calendar clients subscribed to the lecture hall feed by URL.
//
// The old course calendar URL (/api/download_ics/...) is still answered, but by apiv2:
// see RegisterLegacyCalendar, wired in cmd/tumlive.
//
// Do not add routes here: new API work belongs in apiv2.
package api

import (
	"github.com/gin-gonic/gin"

	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/pkg/runner_manager"
	"github.com/TUM-Dev/gocast/tools"
)

// ConfigGinRouter registers the remaining v1 routes.
func ConfigGinRouter(
	router *gin.Engine,
	manager *runner_manager.Manager,
) {
	daoWrapper := dao.NewDaoWrapper()

	router.Use(tools.ErrorHandler)

	configGinStreamRestRouter(router, daoWrapper)
	configGinLectureHallApiRouter(router, daoWrapper)
	configTokenRouter(router, daoWrapper)
	configSelfstreamRouter(router, daoWrapper, manager)
}
