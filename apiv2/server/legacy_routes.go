package apiv2

import (
	"net/http"

	"github.com/TUM-Dev/gocast/tools"
)

// Registrar is where routes are registered: a *tools.Router, or an *http.ServeMux.
type Registrar interface {
	Handle(pattern string, handler http.Handler)
}

// Routes of the retired v1 REST API (the former api/ package) that something other than
// the SPA still calls, each served at a /api/v2 address (see Handler) and, unchanged, at
// its original one. The callers are scripts, a webhook and calendar apps; none of them
// can be moved in the same release, and none can be relied on to follow a redirect, so
// the old addresses answer in place.
const (
	selfStreamPublishPath       = "POST /api/v2/selfstream/on-publish"
	legacySelfStreamPublishPath = "POST /api/selfstream/onPublish"

	tokenProxyPath       = "POST /api/v2/token/proxy/{token}"
	legacyTokenProxyPath = "POST /api/token/proxy/{token}"

	lectureHallSchedule    = "GET /api/v2/lecture-halls/schedule.ics"
	legacyLectureHallsPath = "GET /api/schedule.ics"
)

// RegisterLegacyRoutes serves the routes of the retired v1 API at their original
// addresses, next to RegisterLegacyCalendar's. They answer from this package's handlers,
// so both addresses of a route behave the same.
func (a *API) RegisterLegacyRoutes(r Registrar) {
	a.RegisterLegacyCalendar(r)
	r.Handle(legacySelfStreamPublishPath, http.HandlerFunc(a.handleSelfStreamPublish))
	r.Handle(legacyTokenProxyPath, http.HandlerFunc(a.handleTokenProxy))
	r.Handle(legacyLectureHallsPath, http.HandlerFunc(a.handleLectureHallSchedule))
}

// writeLegacyError answers the way v1's error middleware did: the RequestError as JSON.
func writeLegacyError(w http.ResponseWriter, status int, message string, err error) {
	tools.WriteJSON(w, status, tools.RequestError{Status: status, CustomMessage: message, Err: err}.ToResponse())
}
