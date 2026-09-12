package axis

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// recorder captures what the camera would have received, so the vendor CGI string can be
// pinned exactly: a typo in it is invisible until someone is standing in a lecture hall.
type recorder struct {
	method   string
	path     string
	rawQuery string
	body     string
}

// newCam starts a stand-in camera and returns a driver pointed at it. Auth is a valid
// user:password pair because protocol.MakeAuthenticatedRequest panics on anything else.
func newCam(t *testing.T, h http.HandlerFunc) (*AxisCam, *recorder) {
	t.Helper()
	rec := &recorder{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		rec.method, rec.path, rec.rawQuery, rec.body = r.Method, r.URL.Path, r.URL.RawQuery, string(b)
		h(w, r)
	}))
	t.Cleanup(srv.Close)
	return NewAxisCam(strings.TrimPrefix(srv.URL, "http://"), "user:password"), rec
}

// offlineCam points the driver at a port that refuses connections, standing in for a
// camera that is powered off.
func offlineCam(t *testing.T) *AxisCam {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	srv.Close()
	return NewAxisCam(strings.TrimPrefix(srv.URL, "http://"), "user:password")
}

func TestSetPreset(t *testing.T) {
	// Negative and out-of-range ids are included deliberately: the driver does no
	// validation, it formats whatever it is given straight into the CGI query.
	tests := []struct {
		name     string
		presetId int
		wantQ    string
	}{
		{name: "preset 1 builds the documented ptz.cgi query", presetId: 1, wantQ: "gotoserverpresetno=1&camera=1"},
		{name: "preset 0 is passed through unchanged", presetId: 0, wantQ: "gotoserverpresetno=0&camera=1"},
		{name: "a negative preset is not rejected", presetId: -1, wantQ: "gotoserverpresetno=-1&camera=1"},
		{name: "an out-of-range preset is not rejected", presetId: 9999, wantQ: "gotoserverpresetno=9999&camera=1"},
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
			if rec.path != "/axis-cgi/com/ptz.cgi" {
				t.Errorf("path = %q, want /axis-cgi/com/ptz.cgi", rec.path)
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
	t.Run("fetches the jpg CGI and stores the bytes under a uuid filename", func(t *testing.T) {
		dir := t.TempDir()
		cam, rec := newCam(t, func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("jpegbytes"))
		})

		filename, err := cam.TakeSnapshot(dir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if rec.path != "/axis-cgi/jpg/image.cgi" || rec.rawQuery != "compression=75" {
			t.Errorf("requested %q?%q, want /axis-cgi/jpg/image.cgi?compression=75", rec.path, rec.rawQuery)
		}
		if !strings.HasSuffix(filename, ".jpg") || len(filename) != len("00000000-0000-0000-0000-000000000000.jpg") {
			t.Errorf("filename = %q, want a uuid with a .jpg suffix", filename)
		}
		content, err := os.ReadFile(filepath.Join(dir, filename))
		if err != nil {
			t.Fatalf("reading snapshot: %v", err)
		}
		if string(content) != "jpegbytes" {
			t.Errorf("stored content = %q, want %q", content, "jpegbytes")
		}
	})

	// An empty or error body is still written to disk: the driver never inspects it.
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

	t.Run("an offline camera returns an error", func(t *testing.T) {
		if _, err := offlineCam(t).TakeSnapshot(t.TempDir()); err == nil {
			t.Error("expected an error from an unreachable camera")
		}
	})
}

func TestGetPresets(t *testing.T) {
	t.Run("posts the param.cgi list action", func(t *testing.T) {
		cam, rec := newCam(t, func(w http.ResponseWriter, r *http.Request) {})
		if _, err := cam.GetPresets(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if rec.method != http.MethodPost {
			t.Errorf("method = %q, want POST", rec.method)
		}
		if rec.path != "/axis-cgi/param.cgi" {
			t.Errorf("path = %q, want /axis-cgi/param.cgi", rec.path)
		}
		if want := "action=list&group=root.PTZ.Preset.P0.Position.*.Name"; rec.body != want {
			t.Errorf("body = %q, want %q", rec.body, want)
		}
	})

	// The response shape is the vendor's, so a well-formed sample is parsed line by line.
	tests := []struct {
		name      string
		body      string
		wantNames []string
		wantIds   []int
		wantErr   bool
	}{
		{
			name:      "a well-formed listing yields one preset per line",
			body:      "root.PTZ.Preset.P0.Position.P1.Name=Tafel\nroot.PTZ.Preset.P0.Position.P2.Name=Publikum\n",
			wantNames: []string{"Tafel", "Publikum"},
			wantIds:   []int{1, 2},
		},
		{
			name:      "a listing without a trailing newline is still parsed",
			body:      "root.PTZ.Preset.P0.Position.P3.Name=Dozent",
			wantNames: []string{"Dozent"},
			wantIds:   []int{3},
		},
		{
			name: "an empty body yields no presets and no error",
			body: "",
		},
		{
			name: "a body of blank lines yields no presets",
			body: "\n\n\n",
		},
		{
			name:    "a line with the wrong number of dotted segments is rejected",
			body:    "root.PTZ.Name=Tafel\n",
			wantErr: true,
		},
		{
			name:    "a non-numeric preset index is rejected",
			body:    "root.PTZ.Preset.P0.Position.Pxx.Name=Tafel\n",
			wantErr: true,
		},
		{
			// A camera answering with an error page rather than key=value: the status is
			// discarded upstream, so this body is all the driver sees.
			name: "an HTML error page yields no presets rather than an error",
			body: "<html><head><title>401 Unauthorized</title></head></html>",
		},
		{
			// Two "=" make the split length 3, so the line is skipped silently.
			name: "a line with two equals signs is skipped",
			body: "root.PTZ.Preset.P0.Position.P1.Name=Tafel=Extra\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cam, _ := newCam(t, func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte(tt.body))
			})
			presets, err := cam.GetPresets()
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected an error, got presets %v", presets)
				}
				if presets != nil {
					t.Errorf("presets = %v, want nil alongside the error", presets)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(presets) != len(tt.wantNames) {
				t.Fatalf("got %d presets, want %d (%v)", len(presets), len(tt.wantNames), presets)
			}
			for i := range presets {
				if presets[i].Name != tt.wantNames[i] {
					t.Errorf("preset %d name = %q, want %q", i, presets[i].Name, tt.wantNames[i])
				}
				if presets[i].PresetID != tt.wantIds[i] {
					t.Errorf("preset %d id = %d, want %d", i, presets[i].PresetID, tt.wantIds[i])
				}
			}
		})
	}

	t.Run("an offline camera returns an error and no presets", func(t *testing.T) {
		presets, err := offlineCam(t).GetPresets()
		if err == nil {
			t.Error("expected an error from an unreachable camera")
		}
		if presets != nil {
			t.Errorf("presets = %v, want nil", presets)
		}
	})
}
