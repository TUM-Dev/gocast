package model

import (
	"database/sql"
	"encoding/json"
	"math"
	"strings"
	"testing"
	"time"

	"gorm.io/gorm"
)

// streamAt builds a stream spanning [now+startOffset, now+endOffset). Every time
// assertion below is relative so the suite cannot rot or fail on a given date.
func streamAt(startOffset, endOffset time.Duration) Stream {
	now := time.Now()
	return Stream{Start: now.Add(startOffset), End: now.Add(endOffset)}
}

func TestStreamIsPast(t *testing.T) {
	tests := []struct {
		name   string
		stream Stream
		want   bool
	}{
		{
			name:   "a stream whose end time has passed is past",
			stream: streamAt(-2*time.Hour, -time.Hour),
			want:   true,
		},
		{
			name:   "a stream ending just moments ago is past",
			stream: streamAt(-2*time.Hour, -time.Second),
			want:   true,
		},
		{
			name:   "a stream still running is not past",
			stream: streamAt(-time.Hour, time.Hour),
			want:   false,
		},
		{
			name:   "a stream yet to start is not past",
			stream: streamAt(time.Hour, 2*time.Hour),
			want:   false,
		},
		{
			// Ended is set by the worker when a live stream stops early; it must win
			// over the scheduled end time or the UI keeps showing a dead player.
			name: "a stream flagged as ended is past even though its slot is still running",
			stream: func() Stream {
				s := streamAt(-time.Hour, time.Hour)
				s.Ended = true
				return s
			}(),
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.stream.IsPast(); got != tt.want {
				t.Errorf("IsPast() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestStreamIsComingUp(t *testing.T) {
	tests := []struct {
		name   string
		stream Stream
		want   bool
	}{
		{
			name:   "a stream starting in ten minutes is coming up",
			stream: streamAt(10*time.Minute, 2*time.Hour),
			want:   true,
		},
		{
			// The 30 minute window is what flips the page from a countdown to a
			// waiting room, so both sides of it are pinned.
			name:   "a stream starting just inside the thirty minute window is coming up",
			stream: streamAt(29*time.Minute, 2*time.Hour),
			want:   true,
		},
		{
			name:   "a stream starting just outside the thirty minute window is not coming up",
			stream: streamAt(31*time.Minute, 2*time.Hour),
			want:   false,
		},
		{
			name:   "a stream whose slot has already begun is still coming up until it ends",
			stream: streamAt(-10*time.Minute, time.Hour),
			want:   true,
		},
		{
			name:   "a past stream is never coming up",
			stream: streamAt(-2*time.Hour, -time.Hour),
			want:   false,
		},
		{
			name: "a live stream is not coming up",
			stream: func() Stream {
				s := streamAt(-10*time.Minute, time.Hour)
				s.LiveNow = true
				return s
			}(),
			want: false,
		},
		{
			name: "a recording is not coming up",
			stream: func() Stream {
				s := streamAt(-10*time.Minute, time.Hour)
				s.Recording = true
				return s
			}(),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.stream.IsComingUp(); got != tt.want {
				t.Errorf("IsComingUp() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestStreamTimeSlotReached(t *testing.T) {
	tests := []struct {
		name   string
		stream Stream
		want   bool
	}{
		{
			name:   "a stream currently in its slot has reached it",
			stream: streamAt(-10*time.Minute, time.Hour),
			want:   true,
		},
		{
			// The one minute of lead time is deliberate: the countdown is hidden
			// slightly before the start so it never renders "0:00".
			name:   "a stream starting in thirty seconds has reached its slot",
			stream: streamAt(30*time.Second, time.Hour),
			want:   true,
		},
		{
			name:   "a stream starting in two minutes has not reached its slot",
			stream: streamAt(2*time.Minute, time.Hour),
			want:   false,
		},
		{
			name:   "a stream that already ended has not reached its slot any more",
			stream: streamAt(-2*time.Hour, -time.Hour),
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.stream.TimeSlotReached(); got != tt.want {
				t.Errorf("TimeSlotReached() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestStreamIsPlanned(t *testing.T) {
	tests := []struct {
		name   string
		stream Stream
		want   bool
	}{
		{
			name:   "a stream scheduled for next week is planned",
			stream: streamAt(7*24*time.Hour, 7*24*time.Hour+time.Hour),
			want:   true,
		},
		{
			name:   "a stream about to start is no longer planned but coming up",
			stream: streamAt(10*time.Minute, time.Hour),
			want:   false,
		},
		{
			name:   "a past stream is not planned",
			stream: streamAt(-2*time.Hour, -time.Hour),
			want:   false,
		},
		{
			name: "a recording is not planned",
			stream: func() Stream {
				s := streamAt(7*24*time.Hour, 7*24*time.Hour+time.Hour)
				s.Recording = true
				return s
			}(),
			want: false,
		},
		{
			name: "a live stream is not planned",
			stream: func() Stream {
				s := streamAt(7*24*time.Hour, 7*24*time.Hour+time.Hour)
				s.LiveNow = true
				return s
			}(),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.stream.IsPlanned(); got != tt.want {
				t.Errorf("IsPlanned() = %v, want %v", got, tt.want)
			}
			// Planned and coming up drive mutually exclusive UI states.
			if tt.stream.IsPlanned() && tt.stream.IsComingUp() {
				t.Error("stream is both planned and coming up")
			}
			if tt.stream.IsPlanned() && tt.stream.IsPast() {
				t.Error("stream is both planned and past")
			}
		})
	}
}

func TestStreamIsConverting(t *testing.T) {
	var empty Stream
	if empty.IsConverting() {
		t.Error("a stream with no transcoding progress reports as converting")
	}

	converting := Stream{TranscodingProgresses: []TranscodingProgress{{Progress: 0}}}
	// Progress zero still means a job exists; keying off the value instead of the
	// slice length would hide a just-queued transcode.
	if !converting.IsConverting() {
		t.Error("a stream with a queued transcode does not report as converting")
	}
}

func TestStreamIsDownloadable(t *testing.T) {
	tests := []struct {
		name   string
		stream Stream
		want   bool
	}{
		{
			name:   "a recording with a combined playlist is downloadable",
			stream: Stream{Recording: true, PlaylistUrl: "https://example.org/comb.m3u8"},
			want:   true,
		},
		{
			name:   "a recording with only a presentation playlist is downloadable",
			stream: Stream{Recording: true, PlaylistUrlPRES: "https://example.org/pres.m3u8"},
			want:   true,
		},
		{
			name:   "a recording with only a camera playlist is downloadable",
			stream: Stream{Recording: true, PlaylistUrlCAM: "https://example.org/cam.m3u8"},
			want:   true,
		},
		{
			name:   "a recording without any playlist is not downloadable",
			stream: Stream{Recording: true},
			want:   false,
		},
		{
			// A live stream carries playlist URLs too; offering them as downloads
			// would hand out a partial file.
			name:   "a live stream with playlists is not downloadable",
			stream: Stream{LiveNow: true, PlaylistUrl: "https://example.org/comb.m3u8"},
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.stream.IsDownloadable(); got != tt.want {
				t.Errorf("IsDownloadable() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestStreamIsSelfStream(t *testing.T) {
	// A zero LectureHallID is how "no lecture hall" is stored, so the zero value of
	// a freshly created stream must read as self-streamed.
	if !(&Stream{}).IsSelfStream() {
		t.Error("a stream without a lecture hall is not treated as a self stream")
	}
	if (&Stream{LectureHallID: 1}).IsSelfStream() {
		t.Error("a lecture hall stream is treated as a self stream")
	}
}

func TestStreamGetStartInSeconds(t *testing.T) {
	future := streamAt(time.Hour, 2*time.Hour)
	if got := future.GetStartInSeconds(); got < 3590 || got > 3600 {
		t.Errorf("GetStartInSeconds() = %d, want roughly 3600", got)
	}

	past := streamAt(-time.Hour, time.Hour)
	if got := past.GetStartInSeconds(); got > -3590 || got < -3600 {
		t.Errorf("GetStartInSeconds() = %d, want roughly -3600", got)
	}

	// Live and VoD both render a player, never a countdown, so the offset is
	// suppressed regardless of the scheduled start.
	live := streamAt(time.Hour, 2*time.Hour)
	live.LiveNow = true
	if got := live.GetStartInSeconds(); got != 0 {
		t.Errorf("GetStartInSeconds() on a live stream = %d, want 0", got)
	}

	vod := streamAt(time.Hour, 2*time.Hour)
	vod.Recording = true
	if got := vod.GetStartInSeconds(); got != 0 {
		t.Errorf("GetStartInSeconds() on a recording = %d, want 0", got)
	}
}

func TestStreamGetVodFiles(t *testing.T) {
	tests := []struct {
		name   string
		stream Stream
		want   []DownloadableVod
	}{
		{
			// The order is the order of the download dropdown entries.
			name: "a stream with all three sources offers combined, camera and presentation",
			stream: Stream{
				PlaylistUrl:     "https://example.org/comb.m3u8?jwt=a",
				PlaylistUrlCAM:  "https://example.org/cam.m3u8?jwt=b",
				PlaylistUrlPRES: "https://example.org/pres.m3u8?jwt=c",
			},
			want: []DownloadableVod{
				{FriendlyName: "Combined", DownloadURL: "https://example.org/comb.m3u8?jwt=a&download=1"},
				{FriendlyName: "Camera", DownloadURL: "https://example.org/cam.m3u8?jwt=b&download=1"},
				{FriendlyName: "Presentation", DownloadURL: "https://example.org/pres.m3u8?jwt=c&download=1"},
			},
		},
		{
			name:   "a stream with only a presentation source offers just that one",
			stream: Stream{PlaylistUrlPRES: "https://example.org/pres.m3u8?jwt=c"},
			want: []DownloadableVod{
				{FriendlyName: "Presentation", DownloadURL: "https://example.org/pres.m3u8?jwt=c&download=1"},
			},
		},
		{
			// Must be an empty slice rather than nil: templates range over it and the
			// value is marshalled into JSON where null would break the dropdown.
			name:   "a stream with no sources offers nothing",
			stream: Stream{},
			want:   []DownloadableVod{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.stream.GetVodFiles()
			if got == nil {
				t.Fatal("GetVodFiles() returned nil")
			}
			if len(got) != len(tt.want) {
				t.Fatalf("GetVodFiles() = %+v, want %+v", got, tt.want)
			}
			for i := range tt.want {
				if got[i] != tt.want[i] {
					t.Errorf("GetVodFiles()[%d] = %+v, want %+v", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestStreamHLSUrl(t *testing.T) {
	tests := []struct {
		name   string
		stream Stream
		want   string
	}{
		{
			name:   "a plain playlist url is passed through",
			stream: Stream{PlaylistUrl: "https://example.org/stream/playlist.m3u8"},
			want:   "https://example.org/stream/playlist.m3u8",
		},
		{
			// The literal "quality" is stripped so the edge serves the master
			// playlist instead of a fixed rendition.
			name:   "the quality marker is removed from the url",
			stream: Stream{PlaylistUrl: "https://example.org/stream/quality/playlist.m3u8"},
			want:   "https://example.org/stream//playlist.m3u8",
		},
		{
			name:   "a trimmed stream gets wowza start and duration parameters",
			stream: Stream{PlaylistUrl: "https://example.org/p.m3u8", StartOffset: 60, EndOffset: 300},
			want:   "https://example.org/p.m3u8?wowzaplaystart=60&wowzaplayduration=300",
		},
		{
			// Only StartOffset gates the rewrite, so a missing EndOffset silently
			// yields duration 0. Pinned as current behaviour.
			name:   "a start offset without an end offset still produces a zero duration",
			stream: Stream{PlaylistUrl: "https://example.org/p.m3u8", StartOffset: 60},
			want:   "https://example.org/p.m3u8?wowzaplaystart=60&wowzaplayduration=0",
		},
		{
			name:   "a stream without a playlist yields an empty url",
			stream: Stream{},
			want:   "",
		},
		{
			// BUG (asserted as-is, not fixed): with no playlist but an offset set the
			// result is a bare query string rather than an empty url.
			name:   "a stream without a playlist but with an offset yields a bare query string",
			stream: Stream{StartOffset: 60, EndOffset: 300},
			want:   "?wowzaplaystart=60&wowzaplayduration=300",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.stream.HLSUrl(); got != tt.want {
				t.Errorf("HLSUrl() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestStreamAttachments(t *testing.T) {
	s := Stream{Files: []File{
		{Model: gorm.Model{ID: 1}, Type: FILETYPE_ATTACHMENT, Path: "/a/slides.pdf"},
		{Model: gorm.Model{ID: 2}, Type: FILETYPE_THUMB_COMB, Path: "/a/thumb.jpg"},
		{Model: gorm.Model{ID: 3}, Type: FILETYPE_ATTACHMENT, Path: "/a/notes.pdf"},
	}}

	got := s.Attachments()
	if len(got) != 2 || got[0].ID != 1 || got[1].ID != 3 {
		t.Fatalf("Attachments() = %+v, want the two attachment files", got)
	}

	// Empty, not nil: the attachment list is ranged over in templates.
	if empty := (&Stream{}).Attachments(); empty == nil || len(empty) != 0 {
		t.Errorf("Attachments() on a stream without files = %+v, want an empty slice", empty)
	}
}

func TestStreamGetThumbIdForSource(t *testing.T) {
	s := Stream{Files: []File{
		{Model: gorm.Model{ID: 10}, Type: FILETYPE_THUMB_COMB},
		{Model: gorm.Model{ID: 11}, Type: FILETYPE_THUMB_CAM},
		{Model: gorm.Model{ID: 12}, Type: FILETYPE_THUMB_PRES},
	}}

	tests := []struct {
		name   string
		source string
		want   uint
	}{
		{name: "the camera source resolves to the camera sprite", source: "CAM", want: 11},
		{name: "the presentation source resolves to the presentation sprite", source: "PRES", want: 12},
		{name: "the combined source resolves to the combined sprite", source: "COMB", want: 10},
		{
			// Anything unrecognised falls back to combined rather than erroring, which
			// is what keeps the seek preview working for legacy player links.
			name:   "an unknown source falls back to the combined sprite",
			source: "nonsense",
			want:   10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := s.GetThumbIdForSource(tt.source); got != tt.want {
				t.Errorf("GetThumbIdForSource(%q) = %d, want %d", tt.source, got, tt.want)
			}
		})
	}

	t.Run("a stream without thumbnails reports the invalid file id", func(t *testing.T) {
		if got := (&Stream{}).GetThumbIdForSource("COMB"); got != FILETYPE_INVALID {
			t.Errorf("GetThumbIdForSource() = %d, want FILETYPE_INVALID", got)
		}
	})
}

func TestStreamGetLGThumbnail(t *testing.T) {
	file := func(t FileType, path string) File { return File{Type: t, Path: path} }

	tests := []struct {
		name   string
		files  []File
		want   string
		wantOk bool
	}{
		{
			// CAM_PRES is the composed preview and outranks every single-source
			// thumbnail; the precedence chain is the whole point of this function.
			name: "the combined camera and presentation thumbnail wins over all others",
			files: []File{
				file(FILETYPE_THUMB_LG_PRES, "/pres.jpg"),
				file(FILETYPE_THUMB_LG_CAM, "/cam.jpg"),
				file(FILETYPE_THUMB_LG_COMB, "/comb.jpg"),
				file(FILETYPE_THUMB_LG_CAM_PRES, "/campres.jpg"),
			},
			want: "/campres.jpg", wantOk: true,
		},
		{
			name: "the combined thumbnail wins over camera and presentation",
			files: []File{
				file(FILETYPE_THUMB_LG_PRES, "/pres.jpg"),
				file(FILETYPE_THUMB_LG_CAM, "/cam.jpg"),
				file(FILETYPE_THUMB_LG_COMB, "/comb.jpg"),
			},
			want: "/comb.jpg", wantOk: true,
		},
		{
			name: "the camera thumbnail wins over presentation",
			files: []File{
				file(FILETYPE_THUMB_LG_PRES, "/pres.jpg"),
				file(FILETYPE_THUMB_LG_CAM, "/cam.jpg"),
			},
			want: "/cam.jpg", wantOk: true,
		},
		{
			name:  "the presentation thumbnail is used as the last resort",
			files: []File{file(FILETYPE_THUMB_LG_PRES, "/pres.jpg")},
			want:  "/pres.jpg", wantOk: true,
		},
		{
			// Callers must not render an empty src; the error is the signal to fall
			// back to a placeholder image.
			name:  "a stream with only small thumbnails has no large thumbnail",
			files: []File{file(FILETYPE_THUMB_COMB, "/sprite.jpg")},
			want:  "", wantOk: false,
		},
		{
			name:  "a stream without any files has no large thumbnail",
			files: nil,
			want:  "", wantOk: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := Stream{Files: tt.files}
			got, err := s.GetLGThumbnail()
			if (err == nil) != tt.wantOk {
				t.Fatalf("GetLGThumbnail() error = %v, want error: %v", err, !tt.wantOk)
			}
			if got != tt.want {
				t.Errorf("GetLGThumbnail() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestStreamGetLGThumbnailForVideoType(t *testing.T) {
	s := Stream{Files: []File{
		{Type: FILETYPE_THUMB_LG_COMB, Path: "/comb.jpg"},
		{Type: FILETYPE_THUMB_LG_CAM, Path: "/cam.jpg"},
		{Type: FILETYPE_THUMB_LG_PRES, Path: "/pres.jpg"},
	}}

	tests := []struct {
		name      string
		videoType VideoType
		want      string
		wantOk    bool
	}{
		{name: "the combined video type resolves to the combined thumbnail", videoType: VideoTypeCombined, want: "/comb.jpg", wantOk: true},
		{name: "the camera video type resolves to the camera thumbnail", videoType: VideoTypeCamera, want: "/cam.jpg", wantOk: true},
		{name: "the presentation video type resolves to the presentation thumbnail", videoType: VideoTypePresentation, want: "/pres.jpg", wantOk: true},
		{
			// An unmapped video type resolves to FILETYPE_INVALID and then matches
			// nothing here; see the dedicated case below for why that is fragile.
			name:      "an unknown video type has no thumbnail",
			videoType: VideoType("BOGUS"),
			want:      "", wantOk: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := s.GetLGThumbnailForVideoType(tt.videoType)
			if (err == nil) != tt.wantOk {
				t.Fatalf("GetLGThumbnailForVideoType(%q) error = %v, want error: %v", tt.videoType, err, !tt.wantOk)
			}
			if got != tt.want {
				t.Errorf("GetLGThumbnailForVideoType(%q) = %q, want %q", tt.videoType, got, tt.want)
			}
		})
	}

	t.Run("a stream without files has no thumbnail for any video type", func(t *testing.T) {
		if _, err := (&Stream{}).GetLGThumbnailForVideoType(VideoTypeCombined); err == nil {
			t.Error("GetLGThumbnailForVideoType() returned no error for a stream without files")
		}
	})

	// BUG (asserted as-is, not fixed): an unmapped video type looks up the zero
	// FileType, so a file stored with FILETYPE_INVALID is handed back as if it were
	// the requested thumbnail.
	t.Run("an unknown video type matches a file stored with the invalid type", func(t *testing.T) {
		bogus := Stream{Files: []File{{Type: FILETYPE_INVALID, Path: "/not-a-thumbnail"}}}
		got, err := bogus.GetLGThumbnailForVideoType(VideoType("BOGUS"))
		if err != nil || got != "/not-a-thumbnail" {
			t.Errorf("GetLGThumbnailForVideoType() = (%q, %v), want the invalid-typed file (current behaviour)", got, err)
		}
	})
}

func TestStreamGetName(t *testing.T) {
	named := Stream{Name: "Lecture 3: Monads"}
	if got := named.GetName(); got != "Lecture 3: Monads" {
		t.Errorf("GetName() = %q, want the stream name", got)
	}

	// Unnamed streams are common for auto-created TUMOnline events; the fallback
	// must be derived from the start date, not left blank.
	unnamed := Stream{Start: time.Date(2023, time.March, 7, 10, 0, 0, 0, time.UTC)}
	if got := unnamed.GetName(); got != "Lecture: Mar 7, 2023" {
		t.Errorf("GetName() = %q, want the date derived fallback", got)
	}
}

func TestStreamColor(t *testing.T) {
	past := streamAt(-2*time.Hour, -time.Hour)
	upcoming := streamAt(7*24*time.Hour, 7*24*time.Hour+time.Hour)

	tests := []struct {
		name   string
		stream Stream
		want   string
	}{
		{
			name: "a public recording is green",
			stream: func() Stream {
				s := past
				s.Recording = true
				return s
			}(),
			want: "success",
		},
		{
			// Private recordings must be visually distinct; the recording branch is
			// checked first so this is the only place the private flag shows up.
			name: "a private recording is gray",
			stream: func() Stream {
				s := past
				s.Recording, s.Private = true, true
				return s
			}(),
			want: "gray-500",
		},
		{
			name: "a live stream is red",
			stream: func() Stream {
				s := streamAt(-10*time.Minute, time.Hour)
				s.LiveNow = true
				return s
			}(),
			want: "danger",
		},
		{
			// A slot that elapsed without producing a recording is the failure case
			// operators need to spot.
			name:   "a past stream without a recording is a warning",
			stream: past,
			want:   "warn",
		},
		{
			name:   "an upcoming stream is neutral",
			stream: upcoming,
			want:   "info",
		},
		{
			name: "a private upcoming stream is still neutral",
			stream: func() Stream {
				s := upcoming
				s.Private = true
				return s
			}(),
			want: "info",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.stream.Color(); got != tt.want {
				t.Errorf("Color() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestStreamGetSilencesJson(t *testing.T) {
	tests := []struct {
		name   string
		stream Stream
		want   string
	}{
		{
			name: "silences are serialised as start and end pairs",
			stream: Stream{Silences: []Silence{
				{Start: 0, End: 30},
				{Start: 600, End: 615},
			}},
			want: `[{"start":0,"end":30},{"start":600,"end":615}]`,
		},
		{
			// The result is embedded straight into the player page, so the empty case
			// must be an empty JSON array and never "" or "null".
			name:   "a stream without silences serialises to an empty array",
			stream: Stream{},
			want:   "[]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.stream.GetSilencesJson()
			if got != tt.want {
				t.Errorf("GetSilencesJson() = %q, want %q", got, tt.want)
			}
			var parsed []map[string]uint
			if err := json.Unmarshal([]byte(got), &parsed); err != nil {
				t.Errorf("GetSilencesJson() produced invalid JSON: %v", err)
			}
		})
	}
}

func TestStreamGetDescriptionHTML(t *testing.T) {
	t.Run("markdown is rendered to html", func(t *testing.T) {
		s := Stream{Description: "# Title\n\nSome **bold** text."}
		got := s.GetDescriptionHTML()
		if !strings.Contains(got, "<h1>") || !strings.Contains(got, "<strong>bold</strong>") {
			t.Errorf("GetDescriptionHTML() = %q, want rendered markdown", got)
		}
	})

	t.Run("an empty description renders to nothing", func(t *testing.T) {
		if got := (&Stream{}).GetDescriptionHTML(); got != "" {
			t.Errorf("GetDescriptionHTML() = %q, want an empty string", got)
		}
	})

	// Lecturer-supplied descriptions are rendered unescaped into the course page.
	// This is the XSS boundary: anything the sanitiser lets through executes in the
	// viewer's session.
	t.Run("script tags and event handlers are stripped", func(t *testing.T) {
		s := Stream{Description: `Hello <script>alert('xss')</script> <img src=x onerror="alert(1)"> <a href="javascript:alert(1)">click</a>`}
		got := s.GetDescriptionHTML()
		for _, forbidden := range []string{"<script", "onerror", "javascript:", "alert(1)"} {
			if strings.Contains(strings.ToLower(got), strings.ToLower(forbidden)) {
				t.Errorf("GetDescriptionHTML() = %q, still contains %q", got, forbidden)
			}
		}
		if !strings.Contains(got, "Hello") {
			t.Errorf("GetDescriptionHTML() = %q, dropped the benign text", got)
		}
	})

	t.Run("fully qualified links open in a new tab", func(t *testing.T) {
		s := Stream{Description: "See [the docs](https://example.org/docs)."}
		got := s.GetDescriptionHTML()
		if !strings.Contains(got, `target="_blank"`) {
			t.Errorf("GetDescriptionHTML() = %q, want target=_blank on an external link", got)
		}
	})
}

func TestStreamFirstSilenceAsProgress(t *testing.T) {
	hourLong := func(silences ...Silence) Stream {
		start := time.Date(2023, time.March, 7, 10, 0, 0, 0, time.UTC)
		return Stream{Start: start, End: start.Add(time.Hour), Silences: silences}
	}

	tests := []struct {
		name   string
		stream Stream
		want   float64
	}{
		{
			name:   "a silence covering the first six minutes is a tenth of the way in",
			stream: hourLong(Silence{Start: 0, End: 360}),
			want:   0.1,
		},
		{
			name:   "a stream without silences reports no progress",
			stream: hourLong(),
			want:   0,
		},
		{
			// The value is only meaningful as a skip-intro marker, so a silence that
			// does not start at zero must be ignored rather than scaled.
			name:   "a silence that does not begin at the start is ignored",
			stream: hourLong(Silence{Start: 120, End: 360}),
			want:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.stream.FirstSilenceAsProgress()
			if math.Abs(got-tt.want) > 1e-9 {
				t.Errorf("FirstSilenceAsProgress() = %v, want %v", got, tt.want)
			}
		})
	}

	// A zero length slot used to divide by zero and yield +Inf (or NaN when the
	// silence was empty too). Neither is representable in JSON, so the value has to
	// stay finite: both the watch page template and the API embed it directly.
	t.Run("a zero length stream reports no progress", func(t *testing.T) {
		at := time.Date(2023, time.March, 7, 10, 0, 0, 0, time.UTC)
		s := Stream{Start: at, End: at, Silences: []Silence{{Start: 0, End: 360}}}
		if got := s.FirstSilenceAsProgress(); got != 0 {
			t.Errorf("FirstSilenceAsProgress() = %v, want 0", got)
		}
	})

	t.Run("a zero length stream with a zero length silence reports no progress", func(t *testing.T) {
		at := time.Date(2023, time.March, 7, 10, 0, 0, 0, time.UTC)
		s := Stream{Start: at, End: at, Silences: []Silence{{Start: 0, End: 0}}}
		if got := s.FirstSilenceAsProgress(); got != 0 {
			t.Errorf("FirstSilenceAsProgress() = %v, want 0", got)
		}
	})

	// The failure mode that actually bites: json.Marshal refuses +Inf and NaN, so
	// any response carrying the progress would fail to serialise entirely.
	t.Run("the progress always marshals to JSON", func(t *testing.T) {
		at := time.Date(2023, time.March, 7, 10, 0, 0, 0, time.UTC)
		streams := []Stream{
			hourLong(Silence{Start: 0, End: 360}),
			hourLong(),
			{Start: at, End: at, Silences: []Silence{{Start: 0, End: 360}}},
			{Start: at, End: at, Silences: []Silence{{Start: 0, End: 0}}},
			{Start: at, End: at.Add(-time.Hour), Silences: []Silence{{Start: 0, End: 360}}},
		}
		for _, s := range streams {
			got := s.FirstSilenceAsProgress()
			if math.IsInf(got, 0) || math.IsNaN(got) {
				t.Fatalf("FirstSilenceAsProgress() = %v, want a finite value", got)
			}
			if _, err := json.Marshal(got); err != nil {
				t.Errorf("json.Marshal(%v) = %v, want no error", got, err)
			}
		}
	})
}

func TestParsableTimeFormat(t *testing.T) {
	at := time.Date(2023, time.March, 7, 14, 5, 9, 0, time.UTC)
	if got := ParsableTimeFormat(at); got != "2023-03-07 14:05:09" {
		t.Errorf("ParsableTimeFormat() = %q, want the JS parsable layout", got)
	}

	// A zero time must render as the empty string, not "0001-01-01 ...", or the
	// frontend shows a year-one date for streams that never went live.
	if got := ParsableTimeFormat(time.Time{}); got != "" {
		t.Errorf("ParsableTimeFormat(zero) = %q, want an empty string", got)
	}

	s := Stream{Start: at, LiveNowTimestamp: at.Add(time.Minute)}
	if got := s.ParsableStartTime(); got != "2023-03-07 14:05:09" {
		t.Errorf("ParsableStartTime() = %q", got)
	}
	if got := s.ParsableLiveNowTimestamp(); got != "2023-03-07 14:06:09" {
		t.Errorf("ParsableLiveNowTimestamp() = %q", got)
	}
	if got := (&Stream{Start: at}).ParsableLiveNowTimestamp(); got != "" {
		t.Errorf("ParsableLiveNowTimestamp() with no timestamp = %q, want an empty string", got)
	}
}

func TestStreamFriendlyDateAndTime(t *testing.T) {
	// Fixed dates only: these are pure formatters, so nothing here depends on when
	// the suite runs.
	start := time.Date(2023, time.March, 7, 10, 15, 0, 0, time.UTC)
	s := Stream{Start: start, End: start.Add(90 * time.Minute)}

	if got := s.FriendlyDate(); got != "Tue 07.03.2023" {
		t.Errorf("FriendlyDate() = %q", got)
	}
	if got := s.FriendlyTime(); got != "07.03.2023 10:15 - 11:45" {
		t.Errorf("FriendlyTime() = %q", got)
	}
}

func TestStreamFriendlyNextDate(t *testing.T) {
	// Relative to now by necessity: the function compares against the current day.
	// Offsets are kept well away from midnight boundaries in the far-future case.
	t.Run("a stream today is labelled today", func(t *testing.T) {
		start := time.Now()
		s := Stream{Start: start}
		want := start.Format("Today, 15:04")
		if got := s.FriendlyNextDate(); got != want {
			t.Errorf("FriendlyNextDate() = %q, want %q", got, want)
		}
	})

	t.Run("a stream tomorrow is labelled tomorrow", func(t *testing.T) {
		start := time.Now().Add(24 * time.Hour)
		s := Stream{Start: start}
		want := start.Format("Tomorrow, 15:04")
		if got := s.FriendlyNextDate(); got != want {
			t.Errorf("FriendlyNextDate() = %q, want %q", got, want)
		}
	})

	t.Run("a stream further out is labelled with its date", func(t *testing.T) {
		start := time.Now().Add(5 * 24 * time.Hour)
		s := Stream{Start: start}
		want := start.Format("Mon, January 02. 15:04")
		if got := s.FriendlyNextDate(); got != want {
			t.Errorf("FriendlyNextDate() = %q, want %q", got, want)
		}
	})
}

func TestStreamToDTO(t *testing.T) {
	start := time.Now().Add(7 * 24 * time.Hour)

	t.Run("a planned stream carries no downloads", func(t *testing.T) {
		s := Stream{
			Model:       gorm.Model{ID: 5},
			Name:        "Week 1",
			Description: "Intro",
			Start:       start,
			End:         start.Add(90 * time.Minute),
			RoomCode:    "00.13.009A",
			PlaylistUrl: "https://example.org/p.m3u8",
		}
		dto := s.ToDTO()
		if dto.ID != 5 || dto.Name != "Week 1" || dto.Description != "Intro" {
			t.Errorf("ToDTO() = %+v, want the stream identity fields", dto)
		}
		// Downloads are gated on IsDownloadable; a scheduled stream exposing its
		// playlist as a download would leak an unfinished recording.
		if dto.Downloads != nil {
			t.Errorf("ToDTO().Downloads = %+v, want nil for a non-recording", dto.Downloads)
		}
		if !dto.IsPlanned || dto.IsComingUp || dto.IsRecording {
			t.Errorf("ToDTO() state = planned:%v comingUp:%v recording:%v", dto.IsPlanned, dto.IsComingUp, dto.IsRecording)
		}
		if dto.Duration != 5400 {
			t.Errorf("ToDTO().Duration = %d, want the slot length in seconds", dto.Duration)
		}
		if dto.LectureHall != "00.13.009A" {
			t.Errorf("ToDTO().LectureHall = %q, want the room code", dto.LectureHall)
		}
		if !dto.IsPubliclyVisible {
			t.Error("ToDTO().IsPubliclyVisible = false for a public stream")
		}
	})

	t.Run("a recording carries its downloads and measured duration", func(t *testing.T) {
		s := Stream{
			Start:          start.Add(-14 * 24 * time.Hour),
			End:            start.Add(-14*24*time.Hour + 90*time.Minute),
			Recording:      true,
			Private:        true,
			PlaylistUrl:    "https://example.org/comb.m3u8",
			PlaylistUrlCAM: "https://example.org/cam.m3u8",
			// The measured duration overrides the scheduled slot: recordings routinely
			// run shorter than they were booked for.
			Duration: sql.NullInt32{Int32: 4200, Valid: true},
		}
		dto := s.ToDTO()
		if len(dto.Downloads) != 2 {
			t.Errorf("ToDTO().Downloads = %+v, want both sources", dto.Downloads)
		}
		if dto.Duration != 4200 {
			t.Errorf("ToDTO().Duration = %d, want the measured duration", dto.Duration)
		}
		if dto.IsPubliclyVisible {
			t.Error("ToDTO().IsPubliclyVisible = true for a private stream")
		}
	})
}

func TestStreamGetJson(t *testing.T) {
	start := time.Now().Add(7 * 24 * time.Hour)
	s := Stream{
		Model:         gorm.Model{ID: 3},
		Name:          "Week 2",
		CourseID:      9,
		LectureHallID: 2,
		Start:         start,
		End:           start.Add(time.Hour),
		PlaylistUrl:   "https://example.org/comb.m3u8",
		Files:         []File{{Model: gorm.Model{ID: 1}, Type: FILETYPE_ATTACHMENT, Path: "/x/CAM.mp4"}},
		VideoSections: []VideoSection{{Model: gorm.Model{ID: 4}, Description: "Intro", StartHours: 0, StartMinutes: 1, StartSeconds: 2, FileID: 1}},
	}
	halls := []LectureHall{{Model: gorm.Model{ID: 1}, Name: "HS1"}, {Model: gorm.Model{ID: 2}, Name: "HS2"}}

	got := s.GetJson(halls, &Course{Slug: "eidi"})

	if got["lectureId"] != uint(3) || got["courseId"] != uint(9) || got["courseSlug"] != "eidi" {
		t.Errorf("GetJson() identity fields = %+v", got)
	}
	if got["lectureHallName"] != "HS2" {
		t.Errorf("GetJson()[lectureHallName] = %v, want the matching hall", got["lectureHallName"])
	}
	if got["hasStats"] != false {
		t.Errorf("GetJson()[hasStats] = %v, want false without stats", got["hasStats"])
	}

	// A self stream has no hall to resolve; the placeholder is what the admin UI
	// renders in the lecture hall column.
	self := Stream{Start: start, End: start.Add(time.Hour)}
	if name := self.GetJson(halls, &Course{})["lectureHallName"]; name != "Selfstreaming" {
		t.Errorf("GetJson()[lectureHallName] = %v, want Selfstreaming", name)
	}

	// The payload is marshalled straight into the admin page.
	if _, err := json.Marshal(got); err != nil {
		t.Errorf("GetJson() produced unmarshalable output: %v", err)
	}
}
