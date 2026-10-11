package apiv2

import (
	"crypto/rand"
	"crypto/rsa"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
	"time"

	"go.uber.org/mock/gomock"
	"google.golang.org/grpc/codes"
	"gorm.io/gorm"

	protobuf "github.com/TUM-Dev/gocast/apiv2/protobuf/server"
	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/mock_dao"
	"github.com/TUM-Dev/gocast/model"
	"github.com/TUM-Dev/gocast/pkg/realtimehub"
	"github.com/TUM-Dev/gocast/tools"
)

// useJWTKey lets the handlers sign playlists.
func useJWTKey(t *testing.T) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(tools.SetJWTKeyForTest(key))
}

// watchEnv is stream 7 of course 1 ("gbs") held in hall 3, with mocks for everything
// the handlers read and a real hub for what they publish.
type watchEnv struct {
	api      *API
	stream   *model.Stream
	course   *model.Course
	hall     *model.LectureHall
	progress *mock_dao.MockProgressDao
	subs     *mock_dao.MockSubtitlesDao
	react    *mock_dao.MockStreamReactionDao
	seeks    *mock_dao.MockVideoSeekDao
	audits   *mock_dao.MockAuditDao
	events   <-chan *protobuf.RealtimeEvent
	hub      realtimehub.Hub
}

func newWatchEnv(t *testing.T) *watchEnv {
	t.Helper()
	useJWTKey(t)
	ctrl := gomock.NewController(t)

	env := &watchEnv{
		stream: &model.Stream{
			Model: gorm.Model{ID: 7}, CourseID: 1, LectureHallID: 3, Recording: true,
			Start: time.Now().Add(-48 * time.Hour), End: time.Now().Add(-47 * time.Hour),
			PlaylistUrl: "https://edge/comb.m3u8", PlaylistUrlPRES: "https://edge/pres.m3u8", PlaylistUrlCAM: "https://edge/cam.m3u8",
			Duration: sql.NullInt32{Int32: 3000, Valid: true},
		},
		course: &model.Course{Model: gorm.Model{ID: 1}, Slug: "gbs", Name: "GBS", Year: 2026, TeachingTerm: "W", Visibility: "loggedin"},
		hall: &model.LectureHall{Model: gorm.Model{ID: 3}, Name: "MI HS1", CameraPresets: []model.CameraPreset{
			{PresetID: 1, Name: "Board", Image: "p1.jpg", LectureHallID: 3, IsDefault: true},
			{PresetID: 2, Name: "Desk", LectureHallID: 3},
		}},
		progress: mock_dao.NewMockProgressDao(ctrl),
		subs:     mock_dao.NewMockSubtitlesDao(ctrl),
		react:    mock_dao.NewMockStreamReactionDao(ctrl),
		seeks:    mock_dao.NewMockVideoSeekDao(ctrl),
		audits:   mock_dao.NewMockAuditDao(ctrl),
	}
	streams := mock_dao.NewMockStreamsDao(ctrl)
	streams.EXPECT().GetStreamByID(gomock.Any(), gomock.Any()).DoAndReturn(func(_ any, id string) (model.Stream, error) {
		if id != "7" {
			return model.Stream{}, gorm.ErrRecordNotFound
		}
		return *env.stream, nil
	}).AnyTimes()
	courses := mock_dao.NewMockCoursesDao(ctrl)
	courses.EXPECT().GetCourseById(gomock.Any(), uint(1)).DoAndReturn(func(any, any) (model.Course, error) { return *env.course, nil }).AnyTimes()
	halls := mock_dao.NewMockLectureHallsDao(ctrl)
	halls.EXPECT().GetLectureHallByID(uint(3)).DoAndReturn(func(uint) (model.LectureHall, error) { return *env.hall, nil }).AnyTimes()

	env.hub = realtimehub.NewMemory(realtimehub.WithViewersInterval(time.Hour))
	events, cancel := env.hub.Subscribe(7)
	t.Cleanup(cancel)
	env.events = events

	env.api = &API{
		dao: dao.DaoWrapper{
			StreamsDao: streams, CoursesDao: courses, LectureHallsDao: halls, ProgressDao: env.progress,
			SubtitlesDao: env.subs, StreamReactionDao: env.react, VideoSeekDao: env.seeks, AuditDao: env.audits,
		},
		log: slog.Default(),
		hub: env.hub,
	}
	return env
}

// next returns the next event that is not the hub's own viewer count.
func (e *watchEnv) next(t *testing.T) *protobuf.RealtimeEvent {
	t.Helper()
	for {
		select {
		case ev := <-e.events:
			if ev.GetViewers() == nil {
				return ev
			}
		case <-time.After(time.Second):
			t.Fatal("no event was published")
			return nil
		}
	}
}

func (e *watchEnv) none(t *testing.T) {
	t.Helper()
	for {
		select {
		case ev := <-e.events:
			if ev.GetViewers() == nil {
				t.Fatalf("published %v, want nothing", ev)
			}
		case <-time.After(50 * time.Millisecond):
			return
		}
	}
}

var (
	student   = &model.User{Model: gorm.Model{ID: 20}, Role: model.StudentType, Courses: []model.Course{{Model: gorm.Model{ID: 1}}}}
	outsider  = &model.User{Model: gorm.Model{ID: 21}, Role: model.StudentType}
	courseAdm = &model.User{Model: gorm.Model{ID: 22}, Role: model.LecturerType, AdministeredCourses: []model.Course{{Model: gorm.Model{ID: 1}}}}
)

func (e *watchEnv) watch(t *testing.T, user *model.User) (*protobuf.StreamWatch, error) {
	t.Helper()
	return e.api.GetStreamWatch(asCaller(user), &protobuf.GetStreamWatchRequest{StreamId: 7, Slug: "gbs"})
}

func (e *watchEnv) mustWatch(t *testing.T, user *model.User) *protobuf.StreamWatch {
	t.Helper()
	w, err := e.watch(t, user)
	if err != nil {
		t.Fatalf("GetStreamWatch: %v", err)
	}
	return w
}

func kinds(views []*protobuf.WatchView) []protobuf.WatchViewKind {
	var out []protobuf.WatchViewKind
	for _, v := range views {
		out = append(out, v.Kind)
	}
	return out
}

func TestGetStreamWatchPolicy(t *testing.T) {
	type tc struct {
		name       string
		visibility string
		private    bool
		slug       string
		user       *model.User
		want       codes.Code
	}
	for _, c := range []tc{
		{"anonymous on a public course", "public", false, "gbs", nil, codes.OK},
		{"anonymous on a hidden course", "hidden", false, "gbs", nil, codes.OK},
		{"anonymous on a login-only course", "loggedin", false, "gbs", nil, codes.PermissionDenied},
		{"anyone signed in on a login-only course", "loggedin", false, "gbs", outsider, codes.OK},
		{"enrolled student", "enrolled", false, "gbs", student, codes.OK},
		{"not enrolled on an enrolled course", "enrolled", false, "gbs", outsider, codes.PermissionDenied},
		{"course admin", "enrolled", false, "gbs", courseAdm, codes.OK},
		{"student on a private stream", "loggedin", true, "gbs", student, codes.PermissionDenied},
		{"anonymous on a private stream of a public course", "public", true, "gbs", nil, codes.PermissionDenied},
		{"admin on a private stream", "loggedin", true, "gbs", courseAdm, codes.OK},
		{"a slug of another course", "public", false, "other", nil, codes.InvalidArgument},
	} {
		t.Run(c.name, func(t *testing.T) {
			env := newWatchEnv(t)
			env.course.Visibility = c.visibility
			env.stream.Private = c.private
			if c.user != nil {
				env.progress.EXPECT().LoadProgress(c.user.ID, []uint{7}).Return(nil, nil).AnyTimes()
			}
			env.subs.EXPECT().GetLanguagesByStreamID(gomock.Any(), uint(7)).Return(nil, nil).AnyTimes()
			_, err := env.api.GetStreamWatch(asCaller(c.user), &protobuf.GetStreamWatchRequest{StreamId: 7, Slug: c.slug})
			wantChatCode(t, err, c.want)
		})
	}

	t.Run("a stream that does not exist", func(t *testing.T) {
		env := newWatchEnv(t)
		_, err := env.api.GetStreamWatch(asCaller(nil), &protobuf.GetStreamWatchRequest{StreamId: 99, Slug: "gbs"})
		wantChatCode(t, err, codes.NotFound)
	})
}

func TestGetStreamWatchViews(t *testing.T) {
	const (
		comb = protobuf.WatchViewKind_WATCH_VIEW_KIND_COMBINED
		pres = protobuf.WatchViewKind_WATCH_VIEW_KIND_PRESENTATION
		cam  = protobuf.WatchViewKind_WATCH_VIEW_KIND_CAMERA
	)
	setup := func(t *testing.T) *watchEnv {
		env := newWatchEnv(t)
		env.course.Visibility = "public"
		env.subs.EXPECT().GetLanguagesByStreamID(gomock.Any(), uint(7)).Return([]string{"de", "en"}, nil).AnyTimes()
		env.progress.EXPECT().LoadProgress(gomock.Any(), []uint{7}).Return(nil, nil).AnyTimes()
		return env
	}

	t.Run("every source of a hall without a source mode, signed for the caller", func(t *testing.T) {
		env := setup(t)
		w := env.mustWatch(t, nil)
		got := kinds(w.Views)
		if len(got) != 3 || got[0] != comb || got[1] != pres || got[2] != cam {
			t.Fatalf("views = %v", got)
		}
		for _, v := range w.Views {
			if !strings.Contains(v.PlaylistUrl, "?jwt=") {
				t.Errorf("%v playlist %q is not signed", v.Kind, v.PlaylistUrl)
			}
		}
		if !w.SplitAvailable {
			t.Error("split should be available")
		}
		if w.DefaultLayout != protobuf.WatchLayout_WATCH_LAYOUT_COMBINED {
			t.Errorf("default layout = %v", w.DefaultLayout)
		}
		if w.Stream.GetLectureHallName() != "MI HS1" || w.Stream.GetDuration() != 3000 {
			t.Errorf("stream = %v", w.Stream)
		}
		if strings.Join(w.SubtitleLanguages, ",") != "de,en" {
			t.Errorf("languages = %v", w.SubtitleLanguages)
		}
	})

	for name, c := range map[string]struct {
		mode      model.SourceMode
		want      []protobuf.WatchViewKind
		split     bool
		preferred string
		layout    protobuf.WatchLayout
	}{
		"presentation only": {model.SourceModePRESOnly, []protobuf.WatchViewKind{pres}, false, "Combined", protobuf.WatchLayout_WATCH_LAYOUT_PRESENTATION},
		"camera only":       {model.SourceModeCAMOnly, []protobuf.WatchViewKind{cam}, false, "Split", protobuf.WatchLayout_WATCH_LAYOUT_CAMERA},
		"all, split wanted": {model.SourceModeCOMB, []protobuf.WatchViewKind{comb, pres, cam}, true, "Split", protobuf.WatchLayout_WATCH_LAYOUT_SPLIT},
		"all, camera":       {model.SourceModeCOMB, []protobuf.WatchViewKind{comb, pres, cam}, true, "Camera", protobuf.WatchLayout_WATCH_LAYOUT_CAMERA},
	} {
		t.Run("source mode: "+name, func(t *testing.T) {
			env := setup(t)
			env.course.SetSourcePreference([]model.SourcePreference{{LectureHallID: 3, SourceMode: c.mode}})
			user := &model.User{Model: gorm.Model{ID: 30}, Settings: []model.UserSetting{{Type: model.LectureView, Value: c.preferred}}}
			w := env.mustWatch(t, user)
			got := kinds(w.Views)
			if len(got) != len(c.want) {
				t.Fatalf("views = %v, want %v", got, c.want)
			}
			for i := range got {
				if got[i] != c.want[i] {
					t.Fatalf("views = %v, want %v", got, c.want)
				}
			}
			if w.SplitAvailable != c.split || w.DefaultLayout != c.layout {
				t.Errorf("split %v layout %v, want %v %v", w.SplitAvailable, w.DefaultLayout, c.split, c.layout)
			}
		})
	}

	t.Run("a source mode of another hall is not applied", func(t *testing.T) {
		env := setup(t)
		env.course.SetSourcePreference([]model.SourcePreference{{LectureHallID: 4, SourceMode: model.SourceModePRESOnly}})
		if n := len(env.mustWatch(t, nil).Views); n != 3 {
			t.Errorf("views = %d", n)
		}
	})

	t.Run("a self-stream ignores source modes", func(t *testing.T) {
		env := setup(t)
		env.stream.LectureHallID = 0
		env.course.SetSourcePreference([]model.SourcePreference{{LectureHallID: 0, SourceMode: model.SourceModePRESOnly}})
		w := env.mustWatch(t, courseAdm)
		if len(w.Views) != 3 || w.Stream.LectureHallName != nil {
			t.Errorf("views %d hall %v", len(w.Views), w.Stream.LectureHallName)
		}
		if w.Viewer.CanReportIssue || w.Admin.CanEndStream || w.Admin.LectureHallId != nil {
			t.Errorf("a self-stream has no hall to report or end: %v %v", w.Viewer, w.Admin)
		}
	})

	t.Run("a stream that is neither live nor a recording has no views", func(t *testing.T) {
		env := setup(t)
		env.stream.Recording = false
		w := env.mustWatch(t, nil)
		if len(w.Views) != 0 || w.DefaultLayout != protobuf.WatchLayout_WATCH_LAYOUT_UNSPECIFIED || w.SplitAvailable {
			t.Errorf("watch = %v", w)
		}
	})

	t.Run("a sprite needs its file and an interval, and a live stream has none", func(t *testing.T) {
		env := setup(t)
		env.stream.ThumbInterval = 10
		env.stream.Files = []model.File{
			{Model: gorm.Model{ID: 55}, Type: model.FILETYPE_THUMB_COMB},
			{Model: gorm.Model{ID: 56}, Type: model.FILETYPE_ATTACHMENT, Filename: "slides.pdf", Path: "/mass/slides.pdf"},
			{Model: gorm.Model{ID: 57}, Type: model.FILETYPE_ATTACHMENT, Filename: "wiki", Path: "https://example.org/page"},
		}
		w := env.mustWatch(t, nil)
		if s := w.Views[0].Sprite; s == nil || s.FileId != 55 || s.IntervalSeconds != 10 {
			t.Errorf("combined sprite = %v", s)
		}
		if w.Views[1].Sprite != nil {
			t.Errorf("presentation has no sprite file but got %v", w.Views[1].Sprite)
		}
		if len(w.Attachments) != 2 || w.Attachments[0].ExternalUrl != nil || w.Attachments[1].GetExternalUrl() != "https://example.org/page" {
			t.Errorf("attachments = %v", w.Attachments)
		}

		env.stream.Recording, env.stream.LiveNow = false, true
		if live := env.mustWatch(t, nil); live.Views[0].Sprite != nil || len(live.Silences) != 0 || live.Admin != nil {
			t.Errorf("live = %v", live)
		}
	})

	t.Run("cut offsets are absolute milliseconds, shown as seconds and absent at 0", func(t *testing.T) {
		env := setup(t)
		w := env.mustWatch(t, nil)
		if w.StartOffset != nil || w.EndOffset != nil {
			t.Errorf("uncut: %v %v", w.StartOffset, w.EndOffset)
		}
		env.stream.StartOffset, env.stream.EndOffset = 90_000, 2_700_500
		w = env.mustWatch(t, nil)
		if w.GetStartOffset() != 90 || w.GetEndOffset() != 2700 {
			t.Errorf("offsets = %v %v", w.StartOffset, w.EndOffset)
		}
		env.stream.StartOffset = 0
		if w = env.mustWatch(t, nil); w.StartOffset != nil || w.GetEndOffset() != 2700 {
			t.Errorf("end only: %v %v", w.StartOffset, w.EndOffset)
		}
	})
}

func TestGetStreamWatchDownloads(t *testing.T) {
	for _, c := range []struct {
		name      string
		downloads bool
		user      *model.User
		recording bool
		want      int
	}{
		{"course allows, signed in", true, outsider, true, 3},
		{"course allows, anonymous", true, nil, true, 0},
		{"course forbids, student", false, outsider, true, 0},
		{"course forbids, admin", false, courseAdm, true, 3},
		{"not a recording", true, courseAdm, false, 0},
	} {
		t.Run(c.name, func(t *testing.T) {
			env := newWatchEnv(t)
			env.course.Visibility = "public"
			env.course.DownloadsEnabled = c.downloads
			env.stream.Recording = c.recording
			env.stream.LiveNow = !c.recording
			env.subs.EXPECT().GetLanguagesByStreamID(gomock.Any(), uint(7)).Return(nil, nil).AnyTimes()
			env.progress.EXPECT().LoadProgress(gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()
			w := env.mustWatch(t, c.user)
			if len(w.Downloads) != c.want {
				t.Fatalf("downloads = %v, want %d", w.Downloads, c.want)
			}
			for _, d := range w.Downloads {
				if !strings.HasSuffix(d.Url, "&download=1") || !strings.Contains(d.Url, "?jwt=") {
					t.Errorf("download %q", d.Url)
				}
			}
		})
	}

	t.Run("the source mode applies to downloads", func(t *testing.T) {
		env := newWatchEnv(t)
		env.course.DownloadsEnabled = true
		env.course.SetSourcePreference([]model.SourcePreference{{LectureHallID: 3, SourceMode: model.SourceModeCAMOnly}})
		env.subs.EXPECT().GetLanguagesByStreamID(gomock.Any(), uint(7)).Return(nil, nil)
		env.progress.EXPECT().LoadProgress(gomock.Any(), gomock.Any()).Return(nil, nil)
		w := env.mustWatch(t, student)
		if len(w.Downloads) != 1 || w.Downloads[0].FriendlyName != "Camera" {
			t.Errorf("downloads = %v", w.Downloads)
		}
	})

	t.Run("the signed playlist carries the download permission", func(t *testing.T) {
		for _, allowed := range []bool{true, false} {
			env := newWatchEnv(t)
			env.course.DownloadsEnabled = allowed
			env.subs.EXPECT().GetLanguagesByStreamID(gomock.Any(), uint(7)).Return(nil, nil)
			env.progress.EXPECT().LoadProgress(gomock.Any(), gomock.Any()).Return(nil, nil)
			w := env.mustWatch(t, student)
			_, tok, _ := strings.Cut(w.Views[0].PlaylistUrl, "?jwt=")
			payload := strings.Split(tok, ".")[1]
			raw, err := decodeSegment(payload)
			if err != nil {
				t.Fatal(err)
			}
			var claims struct{ Download bool }
			if err := json.Unmarshal(raw, &claims); err != nil {
				t.Fatal(err)
			}
			if claims.Download != allowed {
				t.Errorf("download claim = %v, want %v", claims.Download, allowed)
			}
		}
	})
}

func TestGetStreamWatchViewer(t *testing.T) {
	env := newWatchEnv(t)
	env.course.Visibility = "public"
	env.course.ChatEnabled = true
	env.course.ModeratedChatEnabled = true
	env.course.AnonymousChatEnabled = false
	env.stream.Silences = []model.Silence{{Start: 0, End: 300}, {Start: 900, End: 950}}
	env.stream.Start = time.Now().Add(-time.Hour)
	env.stream.End = env.stream.Start.Add(time.Hour)
	env.subs.EXPECT().GetLanguagesByStreamID(gomock.Any(), uint(7)).Return(nil, nil).AnyTimes()
	env.progress.EXPECT().LoadProgress(uint(20), []uint{7}).Return([]model.StreamProgress{{StreamID: 7, UserID: 20, Progress: 0.4, Watched: true}}, nil).AnyTimes()
	env.progress.EXPECT().LoadProgress(uint(21), []uint{7}).Return(nil, nil).AnyTimes()

	t.Run("a signed-in viewer has progress and the first silence", func(t *testing.T) {
		w := env.mustWatch(t, student)
		if w.Progress == nil || w.Progress.Progress < 0.39 || !w.Progress.Watched {
			t.Errorf("progress = %v", w.Progress)
		}
		// 300 s of a one hour slot.
		if p := w.GetFirstSilenceEndProgress(); p < 0.083 || p > 0.084 {
			t.Errorf("first silence = %v", p)
		}
		if len(w.Silences) != 2 || w.Silences[1].Start != 900 {
			t.Errorf("silences = %v", w.Silences)
		}
		if !w.Viewer.SignedIn || w.Viewer.IsCourseAdmin || w.Admin != nil {
			t.Errorf("viewer = %v admin = %v", w.Viewer, w.Admin)
		}
		if !w.Chat.Enabled || w.Chat.AnonymousAllowed || !w.Chat.ApprovalRequired {
			t.Errorf("chat = %v", w.Chat)
		}
	})

	t.Run("a viewer without progress still gets a zero one", func(t *testing.T) {
		if w := env.mustWatch(t, outsider); w.Progress == nil || w.Progress.Progress != 0 {
			t.Errorf("progress = %v", w.Progress)
		}
	})

	t.Run("an anonymous viewer has neither", func(t *testing.T) {
		w := env.mustWatch(t, nil)
		if w.Progress != nil || w.FirstSilenceEndProgress != nil || w.Viewer.SignedIn {
			t.Errorf("watch = %v", w)
		}
	})

	t.Run("a first silence that does not start at 0:00 is not skipped", func(t *testing.T) {
		env.stream.Silences = []model.Silence{{Start: 5, End: 300}}
		env.progress.EXPECT().LoadProgress(uint(22), []uint{7}).Return(nil, nil).AnyTimes()
		if w := env.mustWatch(t, courseAdm); w.FirstSilenceEndProgress != nil {
			t.Errorf("first silence = %v", *w.FirstSilenceEndProgress)
		}
	})

	t.Run("reporting needs an alert room, a user and a hall", func(t *testing.T) {
		if env.mustWatch(t, student).Viewer.CanReportIssue {
			t.Error("no alert room configured")
		}
		prev := alertsConfigured
		alertsConfigured = func() bool { return true }
		defer func() { alertsConfigured = prev }()
		env.progress.EXPECT().LoadProgress(gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()
		if !env.mustWatch(t, student).Viewer.CanReportIssue {
			t.Error("a student may report")
		}
		if env.mustWatch(t, nil).Viewer.CanReportIssue {
			t.Error("an anonymous viewer may not")
		}
	})
}

func TestGetStreamWatchAdmin(t *testing.T) {
	setup := func(t *testing.T) *watchEnv {
		env := newWatchEnv(t)
		env.progress.EXPECT().LoadProgress(gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()
		env.subs.EXPECT().GetLanguagesByStreamID(gomock.Any(), uint(7)).Return(nil, nil).AnyTimes()
		return env
	}

	t.Run("presets and ending only while live in a hall", func(t *testing.T) {
		env := setup(t)
		w := env.mustWatch(t, courseAdm)
		if w.Admin == nil || w.Admin.GetLectureHallId() != 3 || len(w.Admin.CameraPresets) != 0 || w.Admin.CanEndStream {
			t.Fatalf("recording admin = %v", w.Admin)
		}

		env.stream.Recording, env.stream.LiveNow = false, true
		w = env.mustWatch(t, courseAdm)
		if !w.Admin.CanEndStream || len(w.Admin.CameraPresets) != 2 {
			t.Fatalf("live admin = %v", w.Admin)
		}
		p := w.Admin.CameraPresets
		if p[0].ImageUrl != "/public/p1.jpg" || !p[0].IsDefault || p[1].ImageUrl != "/public/noPreset.jpg" || p[1].PresetId != 2 {
			t.Errorf("presets = %v", p)
		}
	})

	t.Run("a hall without presets has none to offer", func(t *testing.T) {
		env := setup(t)
		env.stream.Recording, env.stream.LiveNow = false, true
		env.hall.CameraPresets = nil
		if w := env.mustWatch(t, courseAdm); len(w.Admin.CameraPresets) != 0 || !w.Admin.CanEndStream {
			t.Errorf("admin = %v", w.Admin)
		}
	})

	t.Run("a student is never sent the admin block", func(t *testing.T) {
		env := setup(t)
		env.stream.Recording, env.stream.LiveNow = false, true
		if w := env.mustWatch(t, student); w.Admin != nil {
			t.Errorf("admin = %v", w.Admin)
		}
	})
}

func decodeSegment(s string) ([]byte, error) {
	return base64.RawURLEncoding.DecodeString(s)
}
