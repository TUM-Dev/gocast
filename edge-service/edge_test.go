package main

import (
	"crypto/rand"
	"crypto/rsa"
	"mime"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const testURL = "http://localhost/vod/test.mp4/playlist.m3u8"

func BenchmarkValidateToken(b *testing.B) {
	str, err := prepareJWT(time.Hour, testURL)
	if err != nil {
		b.Fatal(err)
	}
	r, _ := http.NewRequest(http.MethodGet, testURL+"?jwt="+str, nil)
	r.URL.Query().Add("jwt", "abc")
	w := httptest.NewRecorder()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		validateToken(w, r, false)
	}
}

func TestValidateTokenSuccess(t *testing.T) {
	str, err := prepareJWT(time.Hour, testURL)
	if err != nil {
		t.Fatal(err)
	}
	r, _ := http.NewRequest("GET", testURL+"?jwt="+str, nil)
	w := httptest.NewRecorder()

	if _, res := validateToken(w, r, false); !res {
		t.Error("validateToken returned false")
	}

	if w.Code != http.StatusOK {
		t.Errorf("expected status code %d, got %d", http.StatusOK, w.Code)
	}
}

func TestValidateTokenNoJWT(t *testing.T) {
	_, err := prepareJWT(time.Hour, testURL)
	if err != nil {
		t.Fatal(err)
	}
	r, _ := http.NewRequest("GET", testURL, nil)
	w := httptest.NewRecorder()

	if _, res := validateToken(w, r, false); res {
		t.Error("validateToken returned true")
	}

	if w.Code != http.StatusForbidden {
		t.Errorf("expected status code %d, got %d", http.StatusForbidden, w.Code)
	}
}

func TestValidateTokenExpiredJWT(t *testing.T) {
	str, err := prepareJWT(-time.Hour, testURL)
	if err != nil {
		t.Fatal(err)
	}
	r, _ := http.NewRequest("GET", testURL+"?jwt="+str, nil)
	w := httptest.NewRecorder()

	if _, res := validateToken(w, r, false); res {
		t.Error("validateToken returned true")
	}

	if w.Code != http.StatusForbidden {
		t.Errorf("expected status code %d, got %d", http.StatusForbidden, w.Code)
	}
}

func TestValidateTokenBadURL(t *testing.T) {
	str, err := prepareJWT(time.Hour, "-- not a url --")
	if err != nil {
		t.Fatal(err)
	}
	r, _ := http.NewRequest("GET", testURL+"?jwt="+str, nil)
	w := httptest.NewRecorder()

	if _, res := validateToken(w, r, false); res {
		t.Error("validateToken returned true")
	}

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status code %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestValidateTokenIncorrectURL(t *testing.T) {
	str, err := prepareJWT(time.Hour, testURL)
	if err != nil {
		t.Fatal(err)
	}
	r, _ := http.NewRequest("GET", "http://localhost/vod/wrong_video.mp4/playlist.m3u8?jwt="+str, nil)
	w := httptest.NewRecorder()

	if _, res := validateToken(w, r, false); res {
		t.Error("validateToken returned true")
	}

	if w.Code != http.StatusForbidden {
		t.Errorf("expected status code %d, got %d", http.StatusForbidden, w.Code)
	}
}

func TestValidateTokenBadJWT(t *testing.T) {
	_, err := prepareJWT(time.Hour, testURL)
	if err != nil {
		t.Fatal(err)
	}
	r, _ := http.NewRequest("GET", "http://localhost/vod/wrong_video.mp4/playlist.m3u8?jwt=abc", nil)
	w := httptest.NewRecorder()

	if _, res := validateToken(w, r, false); res {
		t.Error("validateToken returned true")
	}

	if w.Code != http.StatusForbidden {
		t.Errorf("expected status code %d, got %d", http.StatusForbidden, w.Code)
	}
}

func TestValidateTokenAdminToken(t *testing.T) {
	_, err := prepareJWT(time.Hour, testURL)
	adminToken = "abcd"
	if err != nil {
		t.Fatal(err)
	}
	r, _ := http.NewRequest("GET", "http://localhost/vod/wrong_video.mp4/playlist.m3u8?jwt=abcd", nil)
	w := httptest.NewRecorder()

	if _, res := validateToken(w, r, false); !res {
		t.Error("validateToken returned true")
	}

	if w.Code != http.StatusOK {
		t.Errorf("expected status code %d, got %d", http.StatusOK, w.Code)
	}
}

// prepareJWT creates a signed JWT token and sets the edge-service servers public key as the key to validate the token.
func prepareJWT(exp time.Duration, playlist string) (string, error) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return "", err
	}
	jwtPubKey = &key.PublicKey

	t := jwt.New(jwt.GetSigningMethod("RS256"))

	t.Claims = &JWTPlaylistClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: &jwt.NumericDate{Time: time.Now().Add(exp)},
		},
		UserID:   0,
		Playlist: playlist,
	}
	return t.SignedString(key)
}

func TestGetFileName(t *testing.T) {
	var nilClaims *JWTPlaylistClaims
	if got := nilClaims.GetFileName(); got != "video.mp4" {
		t.Errorf("nil claims: got %q", got)
	}
	c := &JWTPlaylistClaims{StreamID: "75911", Playlist: "https://edge/vod/75911/STREAM_VERSION_COMBINED/playlist.m3u8"}
	if got := c.GetFileName(); got != "75911.mp4" {
		t.Errorf("fallback: got %q", got)
	}
	c.FileName = "Graphs_2025-10-14.mp4"
	if got := c.GetFileName(); got != c.FileName {
		t.Errorf("claim: got %q", got)
	}
}

func TestContentDisposition(t *testing.T) {
	for _, name := range []string{"75911.mp4", "Übung_2025-10-14.mp4", `a"b.mp4`} {
		got := contentDisposition(name)
		if !strings.HasPrefix(got, "attachment;") {
			t.Errorf("%q: got %q", name, got)
		}
		_, params, err := mime.ParseMediaType(got)
		if err != nil || params["filename"] != name {
			t.Errorf("%q does not round-trip: %q (%v)", name, got, err)
		}
	}
}

// restoreGlobals resets the package state the tests below change.
func restoreGlobals(t *testing.T) {
	key, proto, port, admin := jwtPubKey, originProto, originPort, adminToken
	t.Cleanup(func() { jwtPubKey, originProto, originPort, adminToken = key, proto, port, admin })
}

const liveURL = "http://edge/vm123.cit.tum.de/42/STREAM_VERSION_COMBINED/playlist.m3u8"

func TestEdgeHandlerLiveJWT(t *testing.T) {
	restoreGlobals(t)
	str, err := prepareJWT(time.Hour, liveURL)
	if err != nil {
		t.Fatal(err)
	}
	for name, tc := range map[string]struct {
		target string
		want   int
	}{
		"no jwt":          {"/vm123.cit.tum.de/42/STREAM_VERSION_COMBINED/0001.ts", http.StatusForbidden},
		"garbage jwt":     {"/vm123.cit.tum.de/42/STREAM_VERSION_COMBINED/0001.ts?jwt=abc", http.StatusForbidden},
		"other stream":    {"/vm123.cit.tum.de/43/STREAM_VERSION_COMBINED/0001.ts?jwt=" + str, http.StatusForbidden},
		"other runner":    {"/vm999.cit.tum.de/42/STREAM_VERSION_COMBINED/0001.ts?jwt=" + str, http.StatusForbidden},
		"other version":   {"/vm123.cit.tum.de/42/STREAM_VERSION_PRES/0001.ts?jwt=" + str, http.StatusForbidden},
		"playlist no jwt": {"/vm123.cit.tum.de/42/STREAM_VERSION_COMBINED/playlist.m3u8", http.StatusForbidden},
	} {
		t.Run(name, func(t *testing.T) {
			w := httptest.NewRecorder()
			edgeHandler(w, httptest.NewRequest(http.MethodGet, tc.target, nil))
			if w.Code != tc.want {
				t.Errorf("expected %d, got %d", tc.want, w.Code)
			}
		})
	}
}

func TestValidateTokenForLive(t *testing.T) {
	restoreGlobals(t)
	str, err := prepareJWT(time.Hour, liveURL)
	if err != nil {
		t.Fatal(err)
	}
	r, _ := http.NewRequest("GET", "http://edge/vm123.cit.tum.de/42/STREAM_VERSION_COMBINED/0001.ts?jwt="+str, nil)
	if _, ok := validateTokenFor(httptest.NewRecorder(), r, false, true); !ok {
		t.Error("expected live token to be accepted")
	}
}

func TestEdgeHandlerLiveProxyAddsJWT(t *testing.T) {
	restoreGlobals(t)
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("#EXTM3U\n#EXTINF:2.0,\n0001.ts\n0002.ts"))
	}))
	defer origin.Close()
	host, port, _ := strings.Cut(strings.TrimPrefix(origin.URL, "http://"), ":")
	oldPort, oldProto := originPort, originProto
	originPort, originProto = port, "http://"
	defer func() { originPort, originProto = oldPort, oldProto }()

	liveURL := "http://edge/" + host + "/42/STREAM_VERSION_COMBINED/playlist.m3u8"
	str, err := prepareJWT(time.Hour, liveURL)
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	edgeHandler(w, httptest.NewRequest(http.MethodGet, "/"+host+"/42/STREAM_VERSION_COMBINED/playlist.m3u8?jwt="+str, nil))
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body)
	}
	if !strings.Contains(w.Body.String(), "0001.ts?jwt="+str+"\n") || !strings.HasSuffix(w.Body.String(), "0002.ts?jwt="+str) {
		t.Errorf("segments did not get the jwt:\n%s", w.Body)
	}
}

func TestEdgeHandlerLiveExpiredJWT(t *testing.T) {
	restoreGlobals(t)
	str, err := prepareJWT(-time.Hour, liveURL)
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	edgeHandler(w, httptest.NewRequest(http.MethodGet, "/vm123.cit.tum.de/42/STREAM_VERSION_COMBINED/0001.ts?jwt="+str, nil))
	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", w.Code)
	}
}

func TestEdgeHandlerLiveAdminToken(t *testing.T) {
	restoreGlobals(t)
	if _, err := prepareJWT(time.Hour, liveURL); err != nil {
		t.Fatal(err)
	}
	adminToken = "abcd"
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("#EXTM3U\r\n0001.ts\r\n"))
	}))
	defer origin.Close()
	host, port, _ := strings.Cut(strings.TrimPrefix(origin.URL, "http://"), ":")
	originPort, originProto = port, "http://"

	w := httptest.NewRecorder()
	edgeHandler(w, httptest.NewRequest(http.MethodGet, "/"+host+"/42/STREAM_VERSION_COMBINED/playlist.m3u8?jwt=abcd", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body)
	}
	if want := "#EXTM3U\r\n0001.ts?jwt=abcd\r\n"; w.Body.String() != want {
		t.Errorf("got %q, want %q", w.Body.String(), want)
	}
}

func TestEdgeHandlerLiveStripsRange(t *testing.T) {
	restoreGlobals(t)
	var gotRange string
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotRange = r.Header.Get("Range")
		_, _ = w.Write([]byte("0001.ts"))
	}))
	defer origin.Close()
	host, port, _ := strings.Cut(strings.TrimPrefix(origin.URL, "http://"), ":")
	originPort, originProto = port, "http://"
	str, err := prepareJWT(time.Hour, "http://edge/"+host+"/42/STREAM_VERSION_COMBINED/playlist.m3u8")
	if err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest(http.MethodGet, "/"+host+"/42/STREAM_VERSION_COMBINED/playlist.m3u8?jwt="+str, nil)
	r.Header.Set("Range", "bytes=0-10")
	w := httptest.NewRecorder()
	edgeHandler(w, r)
	if gotRange != "" || !strings.Contains(w.Body.String(), "?jwt=") {
		t.Errorf("range %q forwarded or playlist not rewritten: %q", gotRange, w.Body)
	}
}

func TestAddJWTToSegments(t *testing.T) {
	got := addJWTToSegments("#EXTM3U\n#EXTINF:2.0,\n0001.ts\n#EXT-X-ENDLIST", "tok")
	if want := "#EXTM3U\n#EXTINF:2.0,\n0001.ts?jwt=tok\n#EXT-X-ENDLIST"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	if got, want := addJWTToSegments("0001.ts\r\n", "tok"), "0001.ts?jwt=tok\r\n"; got != want {
		t.Errorf("crlf: got %q, want %q", got, want)
	}
}
