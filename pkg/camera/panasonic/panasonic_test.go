package panasonic

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/TUM-Dev/gocast/pkg/camera/protocol"
)

type recorder struct {
	method   string
	path     string
	rawQuery string
}

// newCam starts a stand-in camera and returns a driver pointed at it. Auth is a valid
// user:password pair because protocol.MakeAuthenticatedRequest panics on anything else.
func newCam(t *testing.T, h http.HandlerFunc) (PanasonicCam, *recorder) {
	t.Helper()
	rec := &recorder{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec.method, rec.path, rec.rawQuery = r.Method, r.URL.Path, r.URL.RawQuery
		h(w, r)
	}))
	t.Cleanup(srv.Close)
	return *NewPanasonicCam(strings.TrimPrefix(srv.URL, "http://"), "user:password"), rec
}

func offlineCam(t *testing.T) PanasonicCam {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	srv.Close()
	return *NewPanasonicCam(strings.TrimPrefix(srv.URL, "http://"), "user:password")
}

// assertEmptyDir fails if anything was written into dir. A snapshot that failed must
// leave nothing behind: whatever consumes lecture-hall snapshots cannot tell a stored
// error page from a real JPEG.
func assertEmptyDir(t *testing.T, dir string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading output directory: %v", err)
	}
	if len(entries) != 0 {
		names := make([]string, 0, len(entries))
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Errorf("output directory contains %v, want no file at all", names)
	}
}

func TestSetPreset(t *testing.T) {
	// The driver does no range checking; negative and out-of-range ids are formatted
	// straight into the query, which is pinned here as current behaviour.
	tests := []struct {
		name     string
		presetId int
		wantQ    string
	}{
		{name: "preset 1 builds the documented camctrl query", presetId: 1, wantQ: "preset=1"},
		{name: "preset 0 (Home) is passed through", presetId: 0, wantQ: "preset=0"},
		{name: "a negative preset is not rejected", presetId: -5, wantQ: "preset=-5"},
		{name: "a preset beyond the camera's 100 slots is not rejected", presetId: 500, wantQ: "preset=500"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cam, rec := newCam(t, func(w http.ResponseWriter, r *http.Request) {})
			if err := cam.SetPreset(tt.presetId); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if rec.method != http.MethodGet {
				t.Errorf("method = %q, want GET", rec.method)
			}
			if rec.path != "/cgi-bin/camctrl" {
				t.Errorf("path = %q, want /cgi-bin/camctrl", rec.path)
			}
			if rec.rawQuery != tt.wantQ {
				t.Errorf("query = %q, want %q", rec.rawQuery, tt.wantQ)
			}
		})
	}

	// A camera that answers 401 or 500 has not moved. Reporting that as a successful preset
	// change left an operator watching a stream that never changed angle, with nothing in
	// the logs; the status is now an error naming the code.
	t.Run("a failing camera returns an error", func(t *testing.T) {
		for _, code := range []int{http.StatusInternalServerError, http.StatusUnauthorized} {
			cam, _ := newCam(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(code) })
			err := cam.SetPreset(1)
			if err == nil {
				t.Errorf("code %d: expected an error, the camera did not move", code)
				continue
			}
			if !errors.Is(err, protocol.ErrUnexpectedStatus) {
				t.Errorf("code %d: error %v does not match protocol.ErrUnexpectedStatus", code, err)
			}
			if !strings.Contains(err.Error(), strconv.Itoa(code)) {
				t.Errorf("code %d: error %q does not name the status code", code, err)
			}
		}
	})

	t.Run("an offline camera returns an error", func(t *testing.T) {
		if err := offlineCam(t).SetPreset(1); err == nil {
			t.Error("expected an error from an unreachable camera")
		}
	})
}

func TestTakeSnapshot(t *testing.T) {
	t.Run("fetches view.cgi and stores the bytes under a uuid filename", func(t *testing.T) {
		dir := t.TempDir()
		cam, rec := newCam(t, func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("jpegbytes"))
		})

		filename, err := cam.TakeSnapshot(dir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if rec.path != "/cgi-bin/view.cgi" || rec.rawQuery != "action=snapshot" {
			t.Errorf("requested %q?%q, want /cgi-bin/view.cgi?action=snapshot", rec.path, rec.rawQuery)
		}
		if !strings.HasSuffix(filename, ".jpg") {
			t.Errorf("filename = %q, want a .jpg suffix", filename)
		}
		content, err := os.ReadFile(filepath.Join(dir, filename))
		if err != nil {
			t.Fatalf("reading snapshot: %v", err)
		}
		if string(content) != "jpegbytes" {
			t.Errorf("stored content = %q, want %q", content, "jpegbytes")
		}
	})

	// The regression worth locking down: an error response used to be written to disk as a
	// .jpg and handed back as a valid snapshot filename.
	t.Run("an error response leaves no file on disk", func(t *testing.T) {
		cases := []struct {
			name string
			code int
			body string
		}{
			{name: "500 with an html error page", code: http.StatusInternalServerError, body: "<html><body>Internal Server Error</body></html>"},
			{name: "401 with an html error page", code: http.StatusUnauthorized, body: "<html><head><title>401 Unauthorized</title></head></html>"},
			{name: "401 with an empty body", code: http.StatusUnauthorized, body: ""},
		}
		for _, tt := range cases {
			t.Run(tt.name, func(t *testing.T) {
				dir := t.TempDir()
				cam, _ := newCam(t, func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(tt.code)
					_, _ = w.Write([]byte(tt.body))
				})
				filename, err := cam.TakeSnapshot(dir)
				if err == nil {
					t.Fatalf("expected an error, got filename %q", filename)
				}
				if !errors.Is(err, protocol.ErrUnexpectedStatus) {
					t.Errorf("error %v does not match protocol.ErrUnexpectedStatus", err)
				}
				if filename != "" {
					t.Errorf("filename = %q, want empty on error", filename)
				}
				assertEmptyDir(t, dir)
			})
		}
	})

	// A 200 carrying something that is not a JPEG is still stored: the driver does not
	// inspect the bytes, and that success path is deliberately unchanged.
	t.Run("a 200 with a non-image body is still stored", func(t *testing.T) {
		const body = "<html>not an image</html>"
		dir := t.TempDir()
		cam, _ := newCam(t, func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(body))
		})
		filename, err := cam.TakeSnapshot(dir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		content, err := os.ReadFile(filepath.Join(dir, filename))
		if err != nil {
			t.Fatalf("reading snapshot: %v", err)
		}
		if string(content) != body {
			t.Errorf("stored content = %q, want %q", content, body)
		}
	})

	// An empty 200 body is what these cameras send while booting: unchanged behaviour, a
	// zero-length file and no error.
	t.Run("an empty body still produces a file and no error", func(t *testing.T) {
		dir := t.TempDir()
		cam, _ := newCam(t, func(w http.ResponseWriter, r *http.Request) {})
		filename, err := cam.TakeSnapshot(dir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		info, err := os.Stat(filepath.Join(dir, filename))
		if err != nil {
			t.Fatalf("stat snapshot: %v", err)
		}
		if info.Size() != 0 {
			t.Errorf("size = %d, want 0", info.Size())
		}
	})

	t.Run("an unwritable output directory returns an error and no filename", func(t *testing.T) {
		cam, _ := newCam(t, func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("jpegbytes"))
		})
		filename, err := cam.TakeSnapshot(filepath.Join(t.TempDir(), "missing"))
		if err == nil {
			t.Fatal("expected an error writing into a missing directory")
		}
		if filename != "" {
			t.Errorf("filename = %q, want empty on error", filename)
		}
	})

	t.Run("an offline camera returns an error and leaves no file", func(t *testing.T) {
		dir := t.TempDir()
		filename, err := offlineCam(t).TakeSnapshot(dir)
		if err == nil {
			t.Error("expected an error from an unreachable camera")
		}
		if filename != "" {
			t.Errorf("filename = %q, want empty on error", filename)
		}
		assertEmptyDir(t, dir)
	})
}

// GetPresets is a pure stub list on Panasonic: it never talks to the camera. The names
// reach the admin UI verbatim, so the exact formatting is pinned.
func TestGetPresets(t *testing.T) {
	cam := *NewPanasonicCam("192.0.2.1", "user:password") // never dialled; asserted by the absence of a server

	presets, err := cam.GetPresets()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(presets) != 10 {
		t.Fatalf("got %d presets, want 10", len(presets))
	}
	if presets[0].Name != "Home" {
		t.Errorf("preset 0 name = %q, want %q", presets[0].Name, "Home")
	}
	for i, p := range presets {
		if p.PresetID != i {
			t.Errorf("preset %d has id %d", i, p.PresetID)
		}
		if i == 0 {
			continue
		}
		if want := fmt.Sprintf("Preset #%2d", i); p.Name != want {
			t.Errorf("preset %d name = %q, want %q", i, p.Name, want)
		}
		// No preset is flagged default, unlike the Sony driver; pinned so the
		// inconsistency is noticed if either side changes.
		if p.IsDefault {
			t.Errorf("preset %d is marked default", i)
		}
	}
	if presets[0].IsDefault {
		t.Error("preset 0 (Home) is marked default; it was not before")
	}
}
