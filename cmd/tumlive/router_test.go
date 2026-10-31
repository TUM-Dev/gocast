package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apiv2 "github.com/TUM-Dev/gocast/apiv2/server"
	"github.com/TUM-Dev/gocast/tools"
)

// The routes of the apiv2 package and of web are registered separately and only meet
// in newRouter, where a pattern of one can conflict with a pattern of the other: the
// legacy /api/... addresses sit beside web's "/{slug}" and "/".
func TestNewRouter(t *testing.T) {
	// web.ConfigRouter refuses to start without a SPA build, as the server does.
	if _, err := os.Stat("../../web/spa/index.html"); err != nil {
		t.Skip("no SPA build present; run `make spa`")
	}
	tools.SetErrorPageRenderer(nil)

	var router *tools.Router
	require.NotPanics(t, func() { router = newRouter(apiv2.New(nil)) })

	get := func(method, target string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(method, target, nil))
		return rec
	}

	t.Run("a web route is not shadowed by the API's", func(t *testing.T) {
		rec := get(http.MethodGet, "/logout")
		assert.Equal(t, http.StatusFound, rec.Code)
		assert.Equal(t, "/", rec.Header().Get("Location"))
	})
	t.Run("an unknown API path is the gateway's, not the 404 page", func(t *testing.T) {
		rec := get(http.MethodGet, "/api/v2/no/such/route")
		assert.Equal(t, http.StatusNotFound, rec.Code)
		assert.NotContains(t, rec.Body.String(), "spa-assets")
	})
	t.Run("an unknown path outside the API is the SPA's 404", func(t *testing.T) {
		assert.Equal(t, http.StatusNotFound, get(http.MethodGet, "/no/such/page").Code)
	})
	t.Run("the legacy addresses are routed", func(t *testing.T) {
		// A wrong method on each is the router's, not the page catch-all's: only POST
		// is the webhook, and the catch-all would answer the SPA's 404.
		assert.Equal(t, http.StatusBadRequest, get(http.MethodPost, "/api/selfstream/onPublish").Code)
		assert.Equal(t, http.StatusBadRequest, get(http.MethodPost, "/api/v2/selfstream/on-publish").Code)
	})
}
