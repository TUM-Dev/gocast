package tools

import (
	"crypto/rand"
	"crypto/rsa"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"gorm.io/gorm"

	"github.com/TUM-Dev/gocast/model"
)

// initTestJWTKey generates a fresh RSA key and wires it into the package-level
// jwtKey so that Cfg.GetJWTKey() works during tests (no config file needed).
func initTestJWTKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("rsa.GenerateKey: %v", err)
	}
	jwtKey = key // package-level var in config.go
	return key
}

// parsePlaylistClaims parses a signed JWT string (produced by SignPlaylistURL or
// SetSignedPlaylists) and returns the embedded JWTPlaylistClaims, verifying the
// signature against the given public key.
func parsePlaylistClaims(t *testing.T, key *rsa.PrivateKey, rawJWT string) *JWTPlaylistClaims {
	t.Helper()
	claims := &JWTPlaylistClaims{}
	tok, err := jwt.ParseWithClaims(rawJWT, claims, func(_ *jwt.Token) (interface{}, error) {
		return key.Public(), nil
	})
	if err != nil {
		t.Fatalf("jwt.ParseWithClaims: %v", err)
	}
	if !tok.Valid {
		t.Fatal("parsed JWT token is not valid")
	}
	return claims
}

// extractJWT pulls the jwt= query parameter value from a signed URL.
func extractJWT(t *testing.T, signedURL string) string {
	t.Helper()
	// Find "jwt=" anywhere in the query string.
	const needle = "jwt="
	idx := strings.Index(signedURL, needle)
	if idx == -1 {
		t.Fatalf("no jwt= parameter found in URL %q", signedURL)
	}
	jwtStr := signedURL[idx+len(needle):]
	// If there are further query params after the JWT token, trim them.
	if amp := strings.Index(jwtStr, "&"); amp != -1 {
		jwtStr = jwtStr[:amp]
	}
	return jwtStr
}

// ---------------------------------------------------------------------------
// SignPlaylistURL unit tests
// ---------------------------------------------------------------------------

func TestSignPlaylistURL_EmptyInput(t *testing.T) {
	_ = initTestJWTKey(t)
	result, err := SignPlaylistURL(nil, "", 1, 2, false, time.Hour)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "" {
		t.Fatalf("expected empty string for empty input, got %q", result)
	}
}

func TestSignPlaylistURL_LRZSkip(t *testing.T) {
	_ = initTestJWTKey(t)
	lrzURL := "https://stream.lrz.de/live/stream.m3u8"
	result, err := SignPlaylistURL(nil, lrzURL, 1, 2, false, time.Hour)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// LRZ URLs must be returned unchanged (not signed).
	if result != lrzURL {
		t.Fatalf("LRZ URL should be returned as-is, got %q", result)
	}
	if strings.Contains(result, "jwt=") {
		t.Fatal("LRZ URL must not contain a jwt parameter")
	}
}

func TestSignPlaylistURL_NilUserTreatedAsZero(t *testing.T) {
	key := initTestJWTKey(t)
	url := "https://gocast.example.com/stream.m3u8"

	signedNil, err := SignPlaylistURL(nil, url, 5, 10, false, time.Hour)
	if err != nil {
		t.Fatalf("unexpected error with nil user: %v", err)
	}

	zero := &model.User{}
	zero.ID = 0
	signedZero, err := SignPlaylistURL(zero, url, 5, 10, false, time.Hour)
	if err != nil {
		t.Fatalf("unexpected error with zero-ID user: %v", err)
	}

	claimsNil := parsePlaylistClaims(t, key, extractJWT(t, signedNil))
	claimsZero := parsePlaylistClaims(t, key, extractJWT(t, signedZero))
	if claimsNil.UserID != 0 {
		t.Errorf("nil user: expected UserID=0, got %d", claimsNil.UserID)
	}
	if claimsZero.UserID != 0 {
		t.Errorf("zero user: expected UserID=0, got %d", claimsZero.UserID)
	}
}

func TestSignPlaylistURL_ClaimsPopulatedCorrectly(t *testing.T) {
	key := initTestJWTKey(t)
	url := "https://gocast.example.com/live/1337.m3u8"
	user := &model.User{}
	user.ID = 42

	const streamID uint = 77
	const courseID uint = 88
	ttl := 2 * time.Hour

	before := time.Now()
	signed, err := SignPlaylistURL(user, url, streamID, courseID, true, ttl)
	after := time.Now()
	if err != nil {
		t.Fatalf("SignPlaylistURL error: %v", err)
	}

	claims := parsePlaylistClaims(t, key, extractJWT(t, signed))
	if claims.UserID != 42 {
		t.Errorf("UserID: got %d, want 42", claims.UserID)
	}
	if claims.Playlist != url {
		t.Errorf("Playlist: got %q, want %q", claims.Playlist, url)
	}
	if !claims.Download {
		t.Error("Download: got false, want true")
	}
	if claims.StreamID != "77" {
		t.Errorf("StreamID: got %q, want %q", claims.StreamID, "77")
	}
	if claims.CourseID != "88" {
		t.Errorf("CourseID: got %q, want %q", claims.CourseID, "88")
	}
	// exp must be approximately now + ttl. JWT NumericDate has second precision,
	// so we compare with a 2-second tolerance on each side.
	exp := claims.ExpiresAt.Time
	wantMin := before.Add(ttl).Add(-2 * time.Second)
	wantMax := after.Add(ttl).Add(2 * time.Second)
	if exp.Before(wantMin) || exp.After(wantMax) {
		t.Errorf("exp %v not in [%v, %v]", exp, wantMin, wantMax)
	}
}

func TestSignPlaylistURL_QueryStringAppend(t *testing.T) {
	_ = initTestJWTKey(t)

	// URL without an existing query string → separator must be '?'
	noQuery := "https://gocast.example.com/stream.m3u8"
	signed, err := SignPlaylistURL(nil, noQuery, 1, 2, false, time.Hour)
	if err != nil {
		t.Fatalf("SignPlaylistURL error: %v", err)
	}
	// The separator between the base URL and jwt= must be '?'
	jwtIdx := strings.Index(signed, "jwt=")
	if jwtIdx == -1 {
		t.Fatalf("no jwt= in signed URL %q", signed)
	}
	sep := string(signed[jwtIdx-1])
	if sep != "?" {
		t.Errorf("expected '?' separator for URL without query string, got %q in %q", sep, signed)
	}

	// URL with an existing query string → separator must be '&'
	withQuery := "https://gocast.example.com/stream.m3u8?foo=bar"
	signedQ, err := SignPlaylistURL(nil, withQuery, 1, 2, false, time.Hour)
	if err != nil {
		t.Fatalf("SignPlaylistURL error: %v", err)
	}
	jwtIdxQ := strings.Index(signedQ, "jwt=")
	if jwtIdxQ == -1 {
		t.Fatalf("no jwt= in signed URL %q", signedQ)
	}
	sepQ := string(signedQ[jwtIdxQ-1])
	if sepQ != "&" {
		t.Errorf("expected '&' separator for URL with existing query string, got %q in %q", sepQ, signedQ)
	}
}

// ---------------------------------------------------------------------------
// SetSignedPlaylists regression tests (must be semantically equivalent to the
// 7 h behaviour that existed before the refactor).
// ---------------------------------------------------------------------------

func TestSetSignedPlaylists_SemanticRegression(t *testing.T) {
	key := initTestJWTKey(t)
	user := &model.User{}
	user.ID = 99

	stream := &model.Stream{
		Model:           gorm.Model{ID: 10},
		CourseID:        20,
		PlaylistUrl:     "https://gocast.example.com/comb.m3u8",
		PlaylistUrlCAM:  "https://gocast.example.com/cam.m3u8",
		PlaylistUrlPRES: "https://gocast.example.com/pres.m3u8",
	}

	before := time.Now()
	if err := SetSignedPlaylists(stream, user, true); err != nil {
		t.Fatalf("SetSignedPlaylists error: %v", err)
	}
	after := time.Now()

	for variant, signedURL := range map[string]string{
		"COMB": stream.PlaylistUrl,
		"CAM":  stream.PlaylistUrlCAM,
		"PRES": stream.PlaylistUrlPRES,
	} {
		claims := parsePlaylistClaims(t, key, extractJWT(t, signedURL))
		if claims.UserID != 99 {
			t.Errorf("%s: UserID=%d, want 99", variant, claims.UserID)
		}
		if claims.StreamID != "10" {
			t.Errorf("%s: StreamID=%q, want \"10\"", variant, claims.StreamID)
		}
		if claims.CourseID != "20" {
			t.Errorf("%s: CourseID=%q, want \"20\"", variant, claims.CourseID)
		}
		if !claims.Download {
			t.Errorf("%s: Download=false, want true", variant)
		}
		// exp must be ≈ now + 7h. JWT NumericDate has second precision so we
		// allow a 2-second tolerance on each side.
		exp := claims.ExpiresAt.Time
		wantMin := before.Add(7 * time.Hour).Add(-2 * time.Second)
		wantMax := after.Add(7 * time.Hour).Add(2 * time.Second)
		if exp.Before(wantMin) || exp.After(wantMax) {
			t.Errorf("%s: exp %v not in 7h window [%v, %v]", variant, exp, wantMin, wantMax)
		}
		// URL structure: must start with the original URL followed by '?jwt='
		switch variant {
		case "COMB":
			if !strings.HasPrefix(signedURL, "https://gocast.example.com/comb.m3u8?jwt=") {
				t.Errorf("COMB: unexpected URL prefix: %q", signedURL)
			}
		case "CAM":
			if !strings.HasPrefix(signedURL, "https://gocast.example.com/cam.m3u8?jwt=") {
				t.Errorf("CAM: unexpected URL prefix: %q", signedURL)
			}
		case "PRES":
			if !strings.HasPrefix(signedURL, "https://gocast.example.com/pres.m3u8?jwt=") {
				t.Errorf("PRES: unexpected URL prefix: %q", signedURL)
			}
		}
	}
}

func TestSetSignedPlaylists_EmptySkip(t *testing.T) {
	_ = initTestJWTKey(t)
	stream := &model.Stream{
		Model:           gorm.Model{ID: 1},
		CourseID:        2,
		PlaylistUrl:     "",
		PlaylistUrlCAM:  "",
		PlaylistUrlPRES: "",
	}
	if err := SetSignedPlaylists(stream, nil, false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stream.PlaylistUrl != "" || stream.PlaylistUrlCAM != "" || stream.PlaylistUrlPRES != "" {
		t.Error("empty playlist URLs must remain empty after SetSignedPlaylists")
	}
}

func TestSetSignedPlaylists_LRZSkip(t *testing.T) {
	_ = initTestJWTKey(t)
	lrzURL := "https://stream.lrz.de/live/stream.m3u8"
	stream := &model.Stream{
		Model:           gorm.Model{ID: 1},
		CourseID:        2,
		PlaylistUrl:     lrzURL,
		PlaylistUrlCAM:  "",
		PlaylistUrlPRES: "",
	}
	if err := SetSignedPlaylists(stream, nil, false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stream.PlaylistUrl != lrzURL {
		t.Errorf("LRZ URL must be unchanged, got %q", stream.PlaylistUrl)
	}
	if strings.Contains(stream.PlaylistUrl, "jwt=") {
		t.Error("LRZ URL must not contain a jwt= parameter after SetSignedPlaylists")
	}
}

func TestSetSignedPlaylists_URLWithExistingQuery(t *testing.T) {
	key := initTestJWTKey(t)
	urlWithQuery := "https://gocast.example.com/stream.m3u8?token=abc"
	stream := &model.Stream{
		Model:           gorm.Model{ID: 3},
		CourseID:        4,
		PlaylistUrl:     urlWithQuery,
		PlaylistUrlCAM:  "",
		PlaylistUrlPRES: "",
	}
	if err := SetSignedPlaylists(stream, nil, false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// separator must be '&'
	jwtIdx := strings.Index(stream.PlaylistUrl, "jwt=")
	if jwtIdx == -1 {
		t.Fatalf("no jwt= in signed URL %q", stream.PlaylistUrl)
	}
	sep := string(stream.PlaylistUrl[jwtIdx-1])
	if sep != "&" {
		t.Errorf("expected '&' separator for URL with existing query, got %q in %q", sep, stream.PlaylistUrl)
	}
	// claims should still be valid
	_ = parsePlaylistClaims(t, key, extractJWT(t, stream.PlaylistUrl))
}

func TestSetSignedPlaylists_NilUser(t *testing.T) {
	key := initTestJWTKey(t)
	stream := &model.Stream{
		Model:       gorm.Model{ID: 5},
		CourseID:    6,
		PlaylistUrl: "https://gocast.example.com/stream.m3u8",
	}
	if err := SetSignedPlaylists(stream, nil, false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	claims := parsePlaylistClaims(t, key, extractJWT(t, stream.PlaylistUrl))
	if claims.UserID != 0 {
		t.Errorf("nil user: expected UserID=0, got %d", claims.UserID)
	}
}
