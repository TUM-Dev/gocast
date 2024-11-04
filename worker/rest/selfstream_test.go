package rest

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/TUM-Dev/gocast/worker/cfg"
	log "github.com/sirupsen/logrus"
)

var (
	r        *http.Request
	w        *httptest.ResponseRecorder
	streamID string
	testSlug string
)

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
