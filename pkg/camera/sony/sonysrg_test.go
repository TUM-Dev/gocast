package sony

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/TUM-Dev/gocast/pkg/camera/protocol"
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
