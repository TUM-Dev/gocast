package tools

import (
	"github.com/golang-jwt/jwt/v4"
	"github.com/joschahenningsen/TUM-Live/model"
	"time"
)

type JWTPlaylistClaims struct {
	jwt.RegisteredClaims
	UserID   uint
	Playlist string
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
