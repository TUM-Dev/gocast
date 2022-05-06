package rest

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/joschahenningsen/TUM-Live/worker/cfg"
	"github.com/joschahenningsen/TUM-Live/worker/pb"
	"github.com/joschahenningsen/TUM-Live/worker/worker"
	log "github.com/sirupsen/logrus"
)

var r *http.Request
var w *httptest.ResponseRecorder
var streamID string
var testSlug string

func setup() {
	r = httptest.NewRequest(http.MethodGet, "https://test.de", nil)
	w = httptest.NewRecorder()
	cfg.TempDir = "/recordings"
	streamID = "123"
	testSlug = "Test"
}

func checkReturnCode(t *testing.T, w *httptest.ResponseRecorder, code int) {
	if w.Result().StatusCode != code {
		t.Errorf("Expected status code %d, but was %d", code, w.Result().StatusCode)
	}
}

func checkBadStreamInfo(t *testing.T) {
	key, slug, err := mustGetStreamInfo(r)
	if err == nil || key != "" || slug != "" {
		t.Errorf("Received invalid response from mustGetStreamInfo: "+
			"err %v, key: %s, slug: %s", err, key, slug)
	}
}

func checkValidStreamInfo(t *testing.T, form url.Values) {
	key, slug, err := mustGetStreamInfo(r)
	if err != nil || key != streamID || slug != form.Get("name") || w.Result().StatusCode != http.StatusOK {
		t.Errorf("Received invalid response from mustGetStreamInfo: "+
			"err %v, key: %s, slug: %s, status: %d", err, key, slug, w.Result().StatusCode)
	}
}

func generateFormRequest(t *testing.T, form url.Values) {
	var err error
	r, err = http.NewRequest(http.MethodPost, "https://test.de", strings.NewReader(form.Encode()))
	if err != nil {
		t.Errorf("Could not create mock request due to error: %v", err)
	}
	// Set header for URL encoding
	r.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	w = httptest.NewRecorder() // Reset the recorder since we need it for new requests
}

// TestInitApi tries to trigger a log.Fatal call when starting the tumlive
func TestInitApi(t *testing.T) {
	timeout := time.After(100 * time.Millisecond)
	done := make(chan bool)
	testAddress := "0.1:1313"
	go func() {
		defer func() { log.StandardLogger().ExitFunc = nil }()
		var fatal bool
		log.StandardLogger().ExitFunc = func(int) { fatal = true } // Overwrite exit function of logger for log.Fatal
		InitApi(testAddress)
		if !fatal {
			t.Error("Error")
		}
		done <- true
	}()

	select {
	case <-timeout:
		t.Fatal("TestInitApi didn't finish in time")
	case <-done:
	}
}

// TestNotAllowedMethods tries to send a request with a invalid method to each handler
func TestNotAllowedMethods(t *testing.T) {
	setup()

	// onPublish and onPublishDone should only work with POST
	r.Method = http.MethodGet
	streams.onPublish(w, r)
	checkReturnCode(t, w, http.StatusMethodNotAllowed)
	streams.onPublishDone(w, r)
	checkReturnCode(t, w, http.StatusMethodNotAllowed)

	w = httptest.NewRecorder() // Reset recorder

	// defaultHandler should only work with GET
	r.Method = http.MethodPost
	defaultHandler(w, r)
	checkReturnCode(t, w, http.StatusMethodNotAllowed)
}

// TestDefaultHandler tests whether a missing worker is handled correctly
func TestDefaultHandler(t *testing.T) {
	setup()
	cfg.WorkerID = "abc"
	defaultHandler(w, r)
	checkReturnCode(t, w, http.StatusOK)

	w = httptest.NewRecorder() // Reset recorder

	cfg.WorkerID = ""
	defaultHandler(w, r)
	checkReturnCode(t, w, http.StatusInternalServerError)
}

// TestPublishing checks that a valid request does not cause any errors
func TestPublishing(t *testing.T) {
	setup()
	r.Method = http.MethodPost

	streams.streams[streamID] = worker.HandleSelfStream(&pb.SelfStreamResponse{
		StreamID: 123,
	}, testSlug)

	// Setup POST request
	form := url.Values{}
	form.Set("name", testSlug)
	form.Set("tcurl", "abc?secret="+streamID)

	// Test onPublishDone, should succeed
	generateFormRequest(t, form)
	streams.onPublishDone(w, r)
	checkReturnCode(t, w, http.StatusOK)

	// Test onPublish, should fail due client response
	generateFormRequest(t, form)
	streams.onPublish(w, r)
	checkReturnCode(t, w, http.StatusForbidden)
}

// TestMustGetStreamInfo tests whether invalid and invalid request are handled correctly
func TestMustGetStreamInfo(t *testing.T) {
	setup()
	// Test for non-existing form, using default r
	checkBadStreamInfo(t)

	// Test form with only one value, tcurl is missing
	form := url.Values{}
	form.Set("name", testSlug)

	generateFormRequest(t, form)
	checkBadStreamInfo(t)

	// Test empty secret
	form.Set("tcurl", "rmtp://abc?secret=")
	generateFormRequest(t, form)
	checkBadStreamInfo(t)

	// Test valid secret
	form.Set("tcurl", "abc?secret="+streamID)
	generateFormRequest(t, form)
	checkValidStreamInfo(t, form)
}
