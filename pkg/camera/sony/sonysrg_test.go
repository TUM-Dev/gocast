package sony

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

type recorder struct {
	mu       sync.Mutex
	method   string
	path     string
	rawQuery string
	paths    []string
}

func (r *recorder) record(req *http.Request) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.method, r.path, r.rawQuery = req.Method, req.URL.Path, req.URL.RawQuery
	r.paths = append(r.paths, req.URL.Path)
}

// newCam starts a stand-in camera and returns a driver pointed at it. Auth is a valid
// user:password pair because protocol.MakeAuthenticatedRequest panics on anything else.
func newCam(t *testing.T, h http.HandlerFunc) (*SonySRG, *recorder) {
	t.Helper()
	rec := &recorder{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec.record(r)
		h(w, r)
	}))
	t.Cleanup(srv.Close)
	return NewSonySRG(strings.TrimPrefix(srv.URL, "http://"), "user:password"), rec
}

func offlineCam(t *testing.T) *SonySRG {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	srv.Close()
	return NewSonySRG(strings.TrimPrefix(srv.URL, "http://"), "user:password")
}

func TestSetPreset(t *testing.T) {
	// The driver does no range checking; the SRG-A40 supports 256 presets but nothing
	// here enforces that, which is pinned as current behaviour.
	tests := []struct {
		name     string
		presetId int
		wantQ    string
	}{
		{name: "preset 1 builds the documented presetposition query", presetId: 1, wantQ: "PresetCall=1"},
		{name: "preset 0 is passed through", presetId: 0, wantQ: "PresetCall=0"},
		{name: "a negative preset is not rejected", presetId: -1, wantQ: "PresetCall=-1"},
		{name: "a preset beyond the camera's 256 slots is not rejected", presetId: 1000, wantQ: "PresetCall=1000"},
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
			if rec.path != "/command/presetposition.cgi" {
				t.Errorf("path = %q, want /command/presetposition.cgi", rec.path)
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
	t.Run("fetches oneshotimage1 and stores the bytes under a uuid filename", func(t *testing.T) {
		dir := t.TempDir()
		cam, rec := newCam(t, func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("jpegbytes"))
		})

		filename, err := cam.TakeSnapshot(dir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if rec.path != "/oneshotimage1" {
			t.Errorf("path = %q, want /oneshotimage1", rec.path)
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

// GetPresets probes preset thumbnails one by one and stops at the first non-200, so the
// stop condition is what decides how many presets a lecture hall shows.
func TestGetPresets(t *testing.T) {
	tests := []struct {
		name      string
		okUpTo    int // number of leading probes answered 200
		failCode  int
		wantCount int
	}{
		{name: "all sixteen thumbnails present yields sixteen presets", okUpTo: 16, failCode: http.StatusNotFound, wantCount: 16},
		{name: "probing stops at the first missing thumbnail", okUpTo: 3, failCode: http.StatusNotFound, wantCount: 3},
		{name: "no thumbnails yields no presets and no error", okUpTo: 0, failCode: http.StatusNotFound, wantCount: 0},
		{name: "a 401 on the first probe is treated as no presets rather than an error", okUpTo: 0, failCode: http.StatusUnauthorized, wantCount: 0},
		{name: "a 500 mid-listing truncates the list silently", okUpTo: 2, failCode: http.StatusInternalServerError, wantCount: 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			seen := 0
			cam, rec := newCam(t, func(w http.ResponseWriter, r *http.Request) {
				seen++
				if seen > tt.okUpTo {
					w.WriteHeader(tt.failCode)
					return
				}
				_, _ = w.Write([]byte("thumbnail"))
			})

			presets, err := cam.GetPresets()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(presets) != tt.wantCount {
				t.Fatalf("got %d presets, want %d", len(presets), tt.wantCount)
			}
			for i, p := range presets {
				if p.PresetID != i+1 {
					t.Errorf("preset %d has id %d, want %d", i, p.PresetID, i+1)
				}
				if want := fmt.Sprintf("Preset %d", i+1); p.Name != want {
					t.Errorf("preset %d name = %q, want %q", i, p.Name, want)
				}
				if p.IsDefault != (i == 0) {
					t.Errorf("preset %d IsDefault = %v, want %v", i, p.IsDefault, i == 0)
				}
			}
			// The thumbnail paths are one-based and pinned: an off-by-one here would
			// silently drop or duplicate a preset in the lecture hall UI.
			for i := 0; i < tt.wantCount; i++ {
				if want := fmt.Sprintf("/preset/presetimg%d.jpg", i+1); rec.paths[i] != want {
					t.Errorf("probe %d requested %q, want %q", i, rec.paths[i], want)
				}
			}
		})
	}

	// An offline camera must not error out of the listing; it yields an empty list.
	t.Run("an offline camera yields no presets and no error", func(t *testing.T) {
		presets, err := offlineCam(t).GetPresets()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(presets) != 0 {
			t.Errorf("got %d presets, want 0", len(presets))
		}
	})

	// A 200 with a body that is not a JPEG is accepted: the driver only looks at the
	// status, never at the bytes.
	t.Run("a non-image body is still counted as a preset", func(t *testing.T) {
		cam, _ := newCam(t, func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("<html>not an image</html>"))
		})
		presets, err := cam.GetPresets()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(presets) != 16 {
			t.Errorf("got %d presets, want 16", len(presets))
		}
	})
}
