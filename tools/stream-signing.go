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

// fileNamePart makes v safe to use inside a file name and bounds its length.
func fileNamePart(v string, max int) string {
	v = strings.Trim(unsafeFileNameChars.ReplaceAllString(strings.TrimSpace(v), "-"), "-._")
	if r := []rune(v); len(r) > max {
		v = strings.Trim(string(r[:max]), "-._")
	}
	return v
}

// DownloadFileName returns a human-friendly file name for downloads of s, like
// "eidi_Graphs-and-Trees_2025-10-14_COMB.mp4". Streams usually have no name, which
// gives "eidi_2025-10-14_COMB.mp4". version (COMB, CAM or PRES) is part of the name so
// that the camera and presentation downloads of one lecture don't overwrite each other.
// Anything that could break a Content-Disposition header or a path on the user's disk
// is replaced, and the parts are bounded in length. It returns "" when there is
// nothing to name the file after, and the edge falls back to the stream id.
func DownloadFileName(courseSlug string, s *model.Stream, version string) string {
	date := ""
	if !s.Start.IsZero() {
		date = s.Start.Format("2006-01-02")
	}
	slug, name := fileNamePart(courseSlug, 40), fileNamePart(s.Name, 80)
	if slug == "" && name == "" && date == "" {
		return ""
	}
	parts := make([]string, 0, 4)
	for _, p := range []string{slug, name, date, version} {
		if p != "" {
			parts = append(parts, p)
		}
	}
	return strings.Join(parts, "_") + ".mp4"
}

// SetSignedPlaylists adds a signed jwt to all available playlist urls that indicates that the
// user is allowed to consume the playlist. The method assumes that the user has been pre-authorized and doesn't
// check for permissions. Downloads signed through it are not named after a course, use
// SetSignedPlaylistsForCourse where downloads may be allowed.
func SetSignedPlaylists(s *model.Stream, user *model.User, allowDownloading bool) error {
	return SetSignedPlaylistsForCourse(s, user, "", allowDownloading)
}

// SetSignedPlaylistsForCourse is SetSignedPlaylists that also names downloads after the
// course (courseSlug), as the stream doesn't know it. The slug is only used when
// allowDownloading is set.
func SetSignedPlaylistsForCourse(s *model.Stream, user *model.User, courseSlug string, allowDownloading bool) error {
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
			claims.FileName = DownloadFileName(courseSlug, s, playlist.Type)
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
