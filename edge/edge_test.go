package main

import (
	"crypto/rand"
	"crypto/rsa"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v4"
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

// prepareJWT creates a signed JWT token and sets the edge servers public key as the key to validate the token.
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
