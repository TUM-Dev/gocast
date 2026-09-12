package protocol

import (
	"bytes"
	"crypto/md5"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
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

	// camera.Service.For hands out "" for a camera type missing from the auths map, so an
	// empty credential is a configuration state, not malformed input: the request goes out
	// unauthenticated rather than panicking or erroring.
	t.Run("an empty credential sends an unauthenticated request", func(t *testing.T) {
		auth := ""
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if got := r.Header.Get("Authorization"); got != "" {
				t.Errorf("Authorization header = %q, want none", got)
			}
			_, _ = w.Write([]byte("anon"))
		}))
		defer srv.Close()

		buf, status, err := MakeAuthenticatedRequest(&auth, "GET", "", srv.URL)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if status != http.StatusOK || buf.String() != "anon" {
			t.Errorf("got (%q, %d), want (%q, 200)", buf.String(), status, "anon")
		}
	})

	// A non-empty credential that is not "user:password" is a misconfiguration: it is
	// reported rather than silently downgraded to an unauthenticated request, and it must
	// never panic -- an unrecovered panic in a handler goroutine takes the server down.
	t.Run("a credential without a colon errors instead of panicking", func(t *testing.T) {
		auth := "userwithoutcolon"
		buf, status, err := MakeAuthenticatedRequest(&auth, "GET", "", closedServerURL(t))
		if err == nil {
			t.Fatal("expected an error for a credential without a colon")
		}
		if buf != nil {
			t.Errorf("buffer = %v, want nil", buf)
		}
		if status != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", status)
		}
	})

	t.Run("a credential with an empty password is accepted", func(t *testing.T) {
		auth := "user:"
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

	// A colon is a legal password character, so only the first one separates the pair: the
	// password "pa:ss" must arrive whole rather than truncated to "pa".
	t.Run("a password containing a colon is kept whole", func(t *testing.T) {
		auth := "user:pa:ss"
		const realm, nonce = "camera", "deadbeef"
		var gotUser, gotResponse string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			hdr := r.Header.Get("Authorization")
			if hdr == "" {
				w.Header().Set("WWW-Authenticate", fmt.Sprintf(`Digest realm=%q, nonce=%q`, realm, nonce))
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			gotUser = digestParam(hdr, "username")
			gotResponse = digestParam(hdr, "response")
			_, _ = w.Write([]byte("done"))
		}))
		defer srv.Close()

		if _, _, err := MakeAuthenticatedRequest(&auth, "GET", "", srv.URL); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if gotUser != "user" {
			t.Errorf("username = %q, want %q", gotUser, "user")
		}
		ha1 := md5hex("user:" + realm + ":pa:ss")
		ha2 := md5hex("GET:/")
		if want := md5hex(ha1 + ":" + nonce + ":" + ha2); gotResponse != want {
			t.Errorf("digest response = %q, want %q (password truncated at the colon?)", gotResponse, want)
		}
	})
}

func md5hex(s string) string {
	return fmt.Sprintf("%x", md5.Sum([]byte(s)))
}

// digestParam pulls one quoted parameter out of a Digest Authorization header.
func digestParam(header, key string) string {
	for _, part := range strings.Split(strings.TrimPrefix(header, "Digest "), ",") {
		kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(kv) == 2 && kv[0] == key {
			return strings.Trim(kv[1], `"`)
		}
	}
	return ""
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
