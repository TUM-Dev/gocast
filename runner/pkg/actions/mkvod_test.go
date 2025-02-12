package actions

import "testing"

func TestConvertHLSPlaylist(t *testing.T) {
	input := `#EXTM3U
#EXT-X-VERSION:3
#EXT-X-TARGETDURATION:2
#EXT-X-MEDIA-SEQUENCE:0
#EXT-X-PLAYLIST-TYPE:EVENT
#EXTINF:2.000000,
00000.ts`
	expected := `#EXTM3U
#EXT-X-VERSION:3
#EXT-X-TARGETDURATION:2
#EXT-X-MEDIA-SEQUENCE:0
#EXT-X-PLAYLIST-TYPE:VOD
#EXTINF:2.000000,
00000.ts
#EXT-X-ENDLIST`

	output := vodFromEventPlst(input)
	if output != expected {
		t.Errorf("Expected:\n%s\nGot:\n%s", expected, output)
	}
}
