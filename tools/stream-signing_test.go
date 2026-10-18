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
		slug   string
		stream model.Stream
		want   string
	}{
		{"slug, name and date", "eidi", model.Stream{Name: "Graphs and Trees", Start: day}, "eidi_Graphs-and-Trees_2025-10-14_COMB.mp4"},
		{"streams usually have no name", "eidi", model.Stream{Start: day}, "eidi_2025-10-14_COMB.mp4"},
		{"umlauts are kept", "gad", model.Stream{Name: "Übung: Algorithmen", Start: day}, "gad_Übung-Algorithmen_2025-10-14_COMB.mp4"},
		{"path and header characters are replaced", "a/b", model.Stream{Name: `../a"b/c`, Start: day}, "a-b_a-b-c_2025-10-14_COMB.mp4"},
		{"no slug", "", model.Stream{Name: "Intro", Start: day}, "Intro_2025-10-14_COMB.mp4"},
		{"no date", "eidi", model.Stream{}, "eidi_COMB.mp4"},
		{"nothing to go on", "", model.Stream{}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DownloadFileName(tt.slug, &tt.stream, "COMB"); got != tt.want {
				t.Errorf("DownloadFileName() = %q, want %q", got, tt.want)
			}
		})
	}

	if got := DownloadFileName("eidi", &model.Stream{Start: day}, "PRES"); got != "eidi_2025-10-14_PRES.mp4" {
		t.Errorf("version: got %q", got)
	}

	long := DownloadFileName(strings.Repeat("s", 500), &model.Stream{Name: strings.Repeat("a", 500), Start: day}, "COMB")
	if n := len([]rune(long)); n > 40+1+80+len("_2025-10-14_COMB.mp4") {
		t.Errorf("name not bounded: %d runes", n)
	}
}

// The file name is only read by the edge's download handler, so tokens that can't
// download don't carry it.
func TestSetSignedPlaylistsForCourseFileNameOnlyWhenDownloadable(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	old := jwtKey
	jwtKey = key
	t.Cleanup(func() { jwtKey = old })

	for _, allow := range []bool{true, false} {
		s := &model.Stream{Name: "Intro", Start: time.Date(2025, 10, 14, 9, 0, 0, 0, time.UTC), PlaylistUrlPRES: "https://edge/vod/1/pres/playlist.m3u8"}
		if err := SetSignedPlaylistsForCourse(s, nil, "eidi", allow); err != nil {
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
			want = "eidi_Intro_2025-10-14_PRES.mp4"
		}
		if claims.FileName != want {
			t.Errorf("allowDownloading=%v: FileName = %q, want %q", allow, claims.FileName, want)
		}
	}
}
