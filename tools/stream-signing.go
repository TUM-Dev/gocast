package tools

import (
	"fmt"
	"github.com/golang-jwt/jwt/v4"
	"github.com/joschahenningsen/TUM-Live/model"
	"strings"
	"time"
)

type JWTPlaylistClaims struct {
	jwt.RegisteredClaims
	UserID   uint
	Playlist string
	StreamID string
	CourseID string
}

// SetSignedPlaylists adds a signed jwt to all available playlist urls that indicates that the
// user is allowed to consume the playlist. The method assumes that the user has been pre-authorized and doesn't
// check for permissions.
func SetSignedPlaylists(s *model.Stream, user *model.User) error {
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
		t.Claims = &JWTPlaylistClaims{
			RegisteredClaims: jwt.RegisteredClaims{
				ExpiresAt: &jwt.NumericDate{Time: time.Now().Add(time.Hour * 7)}, // Token expires in 7 hours
			},
			UserID:   userid,
			Playlist: playlist.Playlist,
			StreamID: fmt.Sprintf("%d", s.ID),
			CourseID: fmt.Sprintf("%d", s.CourseID),
		}
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
