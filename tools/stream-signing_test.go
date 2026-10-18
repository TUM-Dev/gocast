package tools

import (
	"crypto/rand"
	"crypto/rsa"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/TUM-Dev/gocast/model"
)

func TestDownloadFileName(t *testing.T) {
	day := time.Date(2025, 10, 14, 9, 0, 0, 0, time.UTC)
	tests := []struct {
		name   string
		stream model.Stream
		want   string
	}{
		{"name and date", model.Stream{Name: "Graphs and Trees", Start: day}, "Graphs-and-Trees_2025-10-14_COMB.mp4"},
		{"umlauts are kept", model.Stream{Name: "Übung: Algorithmen", Start: day}, "Übung-Algorithmen_2025-10-14_COMB.mp4"},
		{"path and header characters are replaced", model.Stream{Name: `../a"b/c`, Start: day}, "a-b-c_2025-10-14_COMB.mp4"},
		{"no name", model.Stream{Start: day}, "2025-10-14_COMB.mp4"},
		{"no date", model.Stream{Name: "Intro"}, "Intro_COMB.mp4"},
		{"nothing to go on", model.Stream{}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DownloadFileName(&tt.stream, "COMB"); got != tt.want {
				t.Errorf("DownloadFileName() = %q, want %q", got, tt.want)
			}
		})
	}

	if got := DownloadFileName(&model.Stream{Name: "Intro", Start: day}, "PRES"); got != "Intro_2025-10-14_PRES.mp4" {
		t.Errorf("version: got %q", got)
	}

	long := DownloadFileName(&model.Stream{Name: strings.Repeat("a", 500), Start: day}, "COMB")
	if n := len([]rune(long)); n > 80+len("_2025-10-14_COMB.mp4") {
		t.Errorf("name not bounded: %d runes", n)
	}
}

// The file name is only read by the edge's download handler, so tokens that can't
// download don't carry it.
func TestSetSignedPlaylistsFileNameOnlyWhenDownloadable(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	old := jwtKey
	jwtKey = key
	t.Cleanup(func() { jwtKey = old })

	for _, allow := range []bool{true, false} {
		s := &model.Stream{Name: "Intro", Start: time.Date(2025, 10, 14, 9, 0, 0, 0, time.UTC), PlaylistUrlPRES: "https://edge/vod/1/pres/playlist.m3u8"}
		if err := SetSignedPlaylists(s, nil, allow); err != nil {
			t.Fatal(err)
		}
		u, err := url.Parse(s.PlaylistUrlPRES)
		if err != nil {
			t.Fatal(err)
		}
		claims := &JWTPlaylistClaims{}
		if _, err := jwt.ParseWithClaims(u.Query().Get("jwt"), claims, func(*jwt.Token) (interface{}, error) { return &key.PublicKey, nil }); err != nil {
			t.Fatal(err)
		}
		want := ""
		if allow {
			want = "Intro_2025-10-14_PRES.mp4"
		}
		if claims.FileName != want {
			t.Errorf("allowDownloading=%v: FileName = %q, want %q", allow, claims.FileName, want)
		}
	}
}
