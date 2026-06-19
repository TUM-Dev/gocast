package tools

import (
	"crypto/rsa"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/TUM-Dev/gocast/model"
)

// SetTestJWTKey overwrites the package-level JWT signing key. It exists ONLY so
// that external test packages (e.g. apiv2/server) can install a deterministic
// signing key without reading a config file.
//
// It deliberately lives in a regular (non-_test.go) source file: a _test.go
// helper would only be visible to tests *within* the tools package, not to
// external test packages that import tools. There is no Go mechanism to expose
// a hook into this package's unexported jwtKey to an external test package
// while also excluding it from the default `go test` / production build without
// a build tag (which the project's build/test commands do not pass). Do NOT
// call this from production code.
func SetTestJWTKey(key *rsa.PrivateKey) {
	jwtKey = key
}

type JWTPlaylistClaims struct {
	jwt.RegisteredClaims
	UserID   uint
	Playlist string
	Download bool
	StreamID string
	CourseID string
}

// SignPlaylistURL signs a single playlist URL and returns the URL with the JWT appended.
// It returns an empty string (and no error) for empty URLs and LRZ-hosted URLs (which
// do not require signing during migration from LRZ services).
// A nil user is treated as userid=0, matching the behaviour of SetSignedPlaylists.
// When the URL already contains a '?' the JWT is appended with '&'; otherwise '?' is used.
func SignPlaylistURL(user *model.User, playlistURL string, streamID, courseID uint, download bool, ttl time.Duration) (string, error) {
	if playlistURL == "" {
		return "", nil
	}
	if strings.Contains(playlistURL, "lrz.de") { // todo: remove after migration from lrz services
		return playlistURL, nil
	}

	var userid uint
	if user != nil {
		userid = user.ID
	}

	t := jwt.New(jwt.GetSigningMethod("RS256"))
	t.Claims = &JWTPlaylistClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: &jwt.NumericDate{Time: time.Now().Add(ttl)},
		},
		UserID:   userid,
		Playlist: playlistURL,
		Download: download,
		StreamID: fmt.Sprintf("%d", streamID),
		CourseID: fmt.Sprintf("%d", courseID),
	}

	str, err := t.SignedString(Cfg.GetJWTKey())
	if err != nil {
		return "", err
	}

	sep := "?"
	if strings.Contains(playlistURL, "?") {
		sep = "&"
	}
	return playlistURL + sep + "jwt=" + str, nil
}

// SetSignedPlaylists adds a signed jwt to all available playlist urls that indicates that the
// user is allowed to consume the playlist. The method assumes that the user has been pre-authorized and doesn't
// check for permissions.
func SetSignedPlaylists(s *model.Stream, user *model.User, allowDownloading bool) error {
	const defaultTTL = 7 * time.Hour

	if s.PlaylistUrl != "" {
		signed, err := SignPlaylistURL(user, s.PlaylistUrl, s.ID, s.CourseID, allowDownloading, defaultTTL)
		if err != nil {
			return err
		}
		s.PlaylistUrl = signed
	}
	if s.PlaylistUrlCAM != "" {
		signed, err := SignPlaylistURL(user, s.PlaylistUrlCAM, s.ID, s.CourseID, allowDownloading, defaultTTL)
		if err != nil {
			return err
		}
		s.PlaylistUrlCAM = signed
	}
	if s.PlaylistUrlPRES != "" {
		signed, err := SignPlaylistURL(user, s.PlaylistUrlPRES, s.ID, s.CourseID, allowDownloading, defaultTTL)
		if err != nil {
			return err
		}
		s.PlaylistUrlPRES = signed
	}
	return nil
}
