package panasonic

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
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

	// BUG: the status code is discarded, so a camera answering 401 or 500 looks like a
	// successful preset change to every caller.
	t.Run("a failing camera is reported as success", func(t *testing.T) {
		for _, code := range []int{http.StatusInternalServerError, http.StatusUnauthorized} {
			cam, _ := newCam(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(code) })
			if err := cam.SetPreset(1); err != nil {
				t.Errorf("code %d: got error %v; if status handling was added, update this test", code, err)
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

	t.Run("an offline camera returns an error", func(t *testing.T) {
		if _, err := offlineCam(t).TakeSnapshot(t.TempDir()); err == nil {
			t.Error("expected an error from an unreachable camera")
		}
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
