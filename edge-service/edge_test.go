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
