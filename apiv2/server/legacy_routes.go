package apiv2

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/TUM-Dev/gocast/tools"
)

// Routes of the retired v1 REST API (the former api/ package) that something other than
// the SPA still calls, each served at a /api/v2 address and, unchanged, at its original
// one. The callers are scripts, a webhook and calendar apps; none of them can be moved
// in the same release, and none can be relied on to follow a redirect, so the old
// addresses answer in place.
const (
	selfStreamPublishPath       = "/api/v2/selfstream/on-publish"
	legacySelfStreamPublishPath = "/api/selfstream/onPublish"

	tokenProxyPrefix       = "/api/v2/token/proxy/"
	legacyTokenProxyPath   = "/api/token/proxy/:token"
	legacyLiveStreamsPath  = "/api/stream/live"
	lectureHallSchedule    = "/api/v2/lecture-halls/schedule.ics"
	legacyLectureHallsPath = "/api/schedule.ics"
)

// RegisterLegacyRoutes serves the routes of the retired v1 API at their original
// addresses, next to RegisterLegacyCalendar's. They answer from this package's handlers,
// so both addresses of a route behave the same.
func (a *API) RegisterLegacyRoutes(r gin.IRoutes) {
	a.RegisterLegacyCalendar(r)
	r.POST(legacySelfStreamPublishPath, a.handleSelfStreamPublish)
	r.POST(legacyTokenProxyPath, func(c *gin.Context) { a.handleTokenProxy(c, c.Param("token")) })
	// Still the admin-token middleware, exactly as in v1: the callers are API users
	// holding such a token. Their move to the API keys of #1996 is the maintainer's to
	// schedule, which is why this route has no v2 address.
	r.GET(legacyLiveStreamsPath, tools.AdminToken(a.dao), a.handleLiveStreams)
	r.GET(legacyLectureHallsPath, a.handleLectureHallSchedule)
}

// legacyRoutes dispatches the v2 addresses of those routes from Proxy; it reports
// whether it handled the request.
func (a *API) legacyRoutes(c *gin.Context) bool {
	r := c.Request
	switch {
	case r.Method == http.MethodPost && r.URL.Path == selfStreamPublishPath:
		a.handleSelfStreamPublish(c)
		return true
	case r.Method == http.MethodGet && r.URL.Path == lectureHallSchedule:
		a.handleLectureHallSchedule(c)
		return true
	case r.Method == http.MethodPost:
		token, ok := strings.CutPrefix(r.URL.Path, tokenProxyPrefix)
		if !ok || token == "" || strings.Contains(token, "/") {
			return false
		}
		a.handleTokenProxy(c, token)
		return true
	}
	return false
}

// abortLegacyError answers the way v1's error middleware did: the RequestError as JSON.
// Written here rather than pushed with c.Error, so the response does not depend on a
// global middleware that only some routers install.
func abortLegacyError(c *gin.Context, status int, message string, err error) {
	c.AbortWithStatusJSON(status, tools.RequestError{Status: status, CustomMessage: message, Err: err}.ToResponse())
}
