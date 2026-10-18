package tools

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/TUM-Dev/gocast/model"
)

type JWTPlaylistClaims struct {
	jwt.RegisteredClaims
	UserID   uint
	Playlist string
	Download bool
	StreamID string
	CourseID string
	// FileName is what a download is saved as. The edge has no database, so the
	// human-friendly name has to travel in the token. Only set when downloads are
	// allowed, since nothing else reads it; also empty for tokens issued before it
	// existed, where the edge falls back to the stream id.
	FileName string `json:",omitempty"`
}

var unsafeFileNameChars = regexp.MustCompile(`[^\p{L}\p{N}._-]+`)

// DownloadFileName returns a human-friendly file name like "Graphs-and-Trees_2025-10-14_COMB.mp4"
// for downloads of s. version (COMB, CAM or PRES) is part of the name so that the
// camera and presentation downloads of one lecture don't overwrite each other.
// Anything that could break a Content-Disposition header or a path on the user's
// disk is replaced, and the result is bounded in length.
func DownloadFileName(s *model.Stream, version string) string {
	name := strings.Trim(unsafeFileNameChars.ReplaceAllString(strings.TrimSpace(s.Name), "-"), "-._")
	if r := []rune(name); len(r) > 80 {
		name = strings.Trim(string(r[:80]), "-._")
	}
	date := ""
	if !s.Start.IsZero() {
		date = s.Start.Format("2006-01-02")
	}
	parts := make([]string, 0, 3)
	for _, p := range []string{name, date, version} {
		if p != "" {
			parts = append(parts, p)
		}
	}
	if name == "" && date == "" {
		return "" // the edge falls back to the stream id
	}
	return strings.Join(parts, "_") + ".mp4"
}

// SetSignedPlaylists adds a signed jwt to all available playlist urls that indicates that the
// user is allowed to consume the playlist. The method assumes that the user has been pre-authorized and doesn't
// check for permissions.
func SetSignedPlaylists(s *model.Stream, user *model.User, allowDownloading bool) error {
	var playlists []struct{ Type, Playlist string }
	if s.PlaylistUrl != "" {
		playlists = append(playlists, struct{ Type, Playlist string }{Type: "COMB", Playlist: s.PlaylistUrl})
	}
	if s.PlaylistUrlCAM != "" {
		playlists = append(playlists, struct{ Type, Playlist string }{Type: "CAM", Playlist: s.PlaylistUrlCAM})
	}
	if s.PlaylistUrlPRES != "" {
		playlists = append(playlists, struct{ Type, Playlist string }{Type: "PRES", Playlist: s.PlaylistUrlPRES})
	}

	for _, playlist := range playlists {
		if strings.Contains(playlist.Playlist, "lrz.de") { // todo: remove after migration from lrz services
			continue
		}

		t := jwt.New(jwt.GetSigningMethod("RS256"))

		var userid uint
		userid = 0
		if user != nil {
			userid = user.ID
		}
		claims := &JWTPlaylistClaims{
			RegisteredClaims: jwt.RegisteredClaims{
				ExpiresAt: &jwt.NumericDate{Time: time.Now().Add(time.Hour * 7)}, // Token expires in 7 hours
			},
			UserID:   userid,
			Playlist: playlist.Playlist,
			Download: allowDownloading,
			StreamID: fmt.Sprintf("%d", s.ID),
			CourseID: fmt.Sprintf("%d", s.CourseID),
		}
		if allowDownloading {
			claims.FileName = DownloadFileName(s, playlist.Type)
		}
		t.Claims = claims
		str, err := t.SignedString(Cfg.GetJWTKey())
		if err != nil {
			return err
		}

		switch playlist.Type {
		case "CAM":
			s.PlaylistUrlCAM += "?jwt=" + str
		case "PRES":
			s.PlaylistUrlPRES += "?jwt=" + str
		case "COMB":
			s.PlaylistUrl += "?jwt=" + str
		}
	}
	return nil
}
