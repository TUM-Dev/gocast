package protocol

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// closedServerURL returns a URL that is guaranteed to refuse connections: an httptest
// server is started to claim a free loopback port, then immediately closed. This stands
// in for a lecture-hall camera that is powered off, without ever touching a real IP.
func closedServerURL(t *testing.T) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	url := srv.URL
	srv.Close()
	return url
}

func TestMakeAuthenticatedRequest(t *testing.T) {
	t.Run("a GET returns the body and status of a healthy camera", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet {
				t.Errorf("method = %q, want GET", r.Method)
			}
			_, _ = w.Write([]byte("OK"))
		}))
		defer srv.Close()

		buf, status, err := MakeAuthenticatedRequest(nil, "GET", "", srv.URL)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if status != http.StatusOK {
			t.Errorf("status = %d, want 200", status)
		}
		if buf.String() != "OK" {
			t.Errorf("body = %q, want %q", buf.String(), "OK")
		}
	})

	t.Run("a POST forwards the body verbatim", func(t *testing.T) {
		const want = "action=list&group=root.PTZ.Preset.P0.Position.*.Name"
		var got string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				t.Errorf("method = %q, want POST", r.Method)
			}
			b, _ := io.ReadAll(r.Body)
			got = string(b)
		}))
		defer srv.Close()

		if _, _, err := MakeAuthenticatedRequest(nil, "POST", want, srv.URL); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != want {
			t.Errorf("forwarded body = %q, want %q", got, want)
		}
	})

	t.Run("an unsupported method is rejected before any connection is made", func(t *testing.T) {
		buf, status, err := MakeAuthenticatedRequest(nil, "PUT", "", closedServerURL(t))
		if err == nil {
			t.Fatal("expected an error for an unsupported method")
		}
		if buf != nil {
			t.Errorf("buffer = %v, want nil", buf)
		}
		if status != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", status)
		}
	})

	t.Run("an unparsable URL errors instead of panicking", func(t *testing.T) {
		buf, status, err := MakeAuthenticatedRequest(nil, "GET", "", "http://%zz/broken")
		if err == nil {
			t.Fatal("expected an error for an unparsable URL")
		}
		if buf != nil || status != http.StatusBadRequest {
			t.Errorf("got (%v, %d), want (nil, 400)", buf, status)
		}
	})

	t.Run("a camera that is offline surfaces a transport error with status 0", func(t *testing.T) {
		buf, status, err := MakeAuthenticatedRequest(nil, "GET", "", closedServerURL(t))
		if err == nil {
			t.Fatal("expected a connection error")
		}
		if buf != nil {
			t.Errorf("buffer = %v, want nil", buf)
		}
		if status != 0 {
			t.Errorf("status = %d, want 0", status)
		}
	})

	// A camera answering 500 or 401 is not reported as an error here; only the status
	// code carries that information. Callers that drop the status (every driver's
	// SetPreset does) therefore cannot tell failure from success -- pinned so the day
	// someone adds status handling, the affected callers are found by a failing test.
	t.Run("a non-200 response is returned without an error", func(t *testing.T) {
		for _, code := range []int{http.StatusInternalServerError, http.StatusUnauthorized, http.StatusNotFound} {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(code)
				_, _ = w.Write([]byte("failure body"))
			}))

			buf, status, err := MakeAuthenticatedRequest(nil, "GET", "", srv.URL)
			if err != nil {
				t.Errorf("code %d: unexpected error: %v", code, err)
			}
			if status != code {
				t.Errorf("status = %d, want %d", status, code)
			}
			if buf == nil || buf.String() != "failure body" {
				t.Errorf("code %d: body not returned", code)
			}
			srv.Close()
		}
	})

	t.Run("an empty body yields an empty buffer and no error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
		defer srv.Close()

		buf, status, err := MakeAuthenticatedRequest(nil, "GET", "", srv.URL)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if status != http.StatusOK {
			t.Errorf("status = %d, want 200", status)
		}
		if buf == nil {
			t.Fatal("buffer is nil for an empty body; callers dereference it")
		}
		if buf.Len() != 0 {
			t.Errorf("body = %q, want empty", buf.String())
		}
	})

	t.Run("a user:password pair still reaches a camera that does not challenge", func(t *testing.T) {
		auth := "user:password"
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("done"))
		}))
		defer srv.Close()

		buf, status, err := MakeAuthenticatedRequest(&auth, "GET", "", srv.URL)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if status != http.StatusOK || buf.String() != "done" {
			t.Errorf("got (%q, %d), want (%q, 200)", buf.String(), status, "done")
		}
	})

	// BUG: the auth string is split on ":" and index 1 is read unconditionally, so any
	// credential without a colon panics. camera.Service.For passes "" for a camera type
	// that has no configured credentials, which makes this reachable from configuration
	// alone. Pinned as current behaviour, not endorsed.
	t.Run("a credential without a colon panics", func(t *testing.T) {
		for _, auth := range []string{"", "userwithoutcolon"} {
			func() {
				defer func() {
					if recover() == nil {
						t.Errorf("auth %q: expected a panic (see BUG note); it no longer panics -- update this test", auth)
					}
				}()
				a := auth
				_, _, _ = MakeAuthenticatedRequest(&a, "GET", "", closedServerURL(t))
			}()
		}
	})
}

func TestSaveResponseBuffer(t *testing.T) {
	t.Run("writes the response bytes to outDir/filename", func(t *testing.T) {
		dir := t.TempDir()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("jpegbytes"))
		}))
		defer srv.Close()

		buf, _, err := MakeAuthenticatedRequest(nil, "GET", "", srv.URL)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if err := SaveResponseBuffer(dir, "snap.jpg", buf); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		content, err := os.ReadFile(filepath.Join(dir, "snap.jpg"))
		if err != nil {
			t.Fatalf("reading written file: %v", err)
		}
		if string(content) != "jpegbytes" {
			t.Errorf("file content = %q, want %q", content, "jpegbytes")
		}
	})

	t.Run("a missing output directory errors instead of panicking", func(t *testing.T) {
		err := SaveResponseBuffer(filepath.Join(t.TempDir(), "does-not-exist"), "snap.jpg", bytes.NewBufferString("ignored"))
		if err == nil {
			t.Error("expected an error writing into a missing directory")
		}
	})
}
