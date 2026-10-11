package apiv2

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/meilisearch/meilisearch-go"
	"go.uber.org/mock/gomock"
	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/types/known/timestamppb"
	"gorm.io/gorm"

	protobuf "github.com/TUM-Dev/gocast/apiv2/protobuf/server"
	"github.com/TUM-Dev/gocast/mock_tools"
	"github.com/TUM-Dev/gocast/model"
	"github.com/TUM-Dev/gocast/pkg/realtimehub"
	"github.com/TUM-Dev/gocast/tools"
	"github.com/TUM-Dev/gocast/tools/bot"
)

// reactions switches the allowed emoji for a test, and keeps the stats ticker from
// firing during it.
func reactions(t *testing.T, list ...string) {
	t.Helper()
	prev, prevInterval := tools.Cfg.AllowedReactions, reactionStatsInterval
	tools.Cfg.AllowedReactions, reactionStatsInterval = list, time.Hour
	t.Cleanup(func() { tools.Cfg.AllowedReactions, reactionStatsInterval = prev, prevInterval })
}

func TestWatchRPCPolicies(t *testing.T) {
	for _, name := range []string{"addStreamReaction", "reportStreamIssue"} {
		if p := methodPolicies["/protobuf.StreamService/"+name]; p != authenticated {
			t.Errorf("%s policy = %+v, want authenticated", name, p)
		}
	}
	if p := methodPolicies["/protobuf.StreamService/endStream"]; !p.courseScoped {
		t.Errorf("endStream policy = %+v, want course-scoped", p)
	}
}

func TestListAllowedStreamReactions(t *testing.T) {
	reactions(t, "👍", "👎")
	env := newWatchEnv(t)

	t.Run("lists the configured emoji for anyone who may watch", func(t *testing.T) {
		env.course.Visibility = "public"
		res, err := env.api.ListAllowedStreamReactions(asCaller(nil), &protobuf.ListAllowedStreamReactionsRequest{StreamId: 7})
		if err != nil || strings.Join(res.Reactions, "") != "👍👎" {
			t.Fatalf("res = %v, err = %v", res, err)
		}
	})
	t.Run("not to someone who may not", func(t *testing.T) {
		env.course.Visibility = "enrolled"
		_, err := env.api.ListAllowedStreamReactions(asCaller(outsider), &protobuf.ListAllowedStreamReactionsRequest{StreamId: 7})
		wantChatCode(t, err, codes.PermissionDenied)
	})
}

func TestAddStreamReaction(t *testing.T) {
	react := func(env *watchEnv, user *model.User, emoji string) error {
		_, err := env.api.AddStreamReaction(asCaller(user), &protobuf.AddStreamReactionRequest{StreamId: 7, Reaction: emoji})
		return err
	}

	t.Run("a first reaction is stored and sent to the administrators only", func(t *testing.T) {
		reactions(t, "👍", "👎")
		env := newWatchEnv(t)
		env.react.EXPECT().GetByStreamAndUser(gomock.Any(), uint(7), uint(20)).Return(model.StreamReaction{}, gorm.ErrRecordNotFound)
		env.react.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, r *model.StreamReaction) error {
			if r.Reaction != "👍" || r.StreamID != 7 || r.UserID != 20 {
				t.Errorf("stored %+v", r)
			}
			return nil
		})

		if err := react(env, student, "👍"); err != nil {
			t.Fatalf("AddStreamReaction: %v", err)
		}
		ev := env.next(t)
		if ev.GetStreamReaction().GetReaction() != "👍" || ev.Audience != protobuf.RealtimeEvent_AUDIENCE_COURSE_ADMINS {
			t.Fatalf("event = %v", ev)
		}
		if !(realtimeViewer{userID: 22, isAdmin: true}).receives(ev) || (realtimeViewer{userID: 20}).receives(ev) || (realtimeViewer{}).receives(ev) {
			t.Error("only administrators receive reactions")
		}
	})

	t.Run("reacting again after the cooldown replaces the reaction", func(t *testing.T) {
		reactions(t, "👍", "👎")
		env := newWatchEnv(t)
		old := model.StreamReaction{Model: gorm.Model{ID: 3, UpdatedAt: time.Now().Add(-time.Minute)}, Reaction: "👍", StreamID: 7, UserID: 20}
		env.react.EXPECT().GetByStreamAndUser(gomock.Any(), uint(7), uint(20)).Return(old, nil)
		env.react.EXPECT().Update(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, r *model.StreamReaction) error {
			if r.ID != 3 || r.Reaction != "👎" {
				t.Errorf("updated %+v", r)
			}
			return nil
		})
		if err := react(env, student, "👎"); err != nil {
			t.Fatal(err)
		}
		env.next(t)
	})

	t.Run("the same emoji again is saved too, so the window moves", func(t *testing.T) {
		reactions(t, "👍")
		env := newWatchEnv(t)
		old := model.StreamReaction{Model: gorm.Model{ID: 3, UpdatedAt: time.Now().Add(-time.Minute)}, Reaction: "👍", StreamID: 7, UserID: 20}
		env.react.EXPECT().GetByStreamAndUser(gomock.Any(), uint(7), uint(20)).Return(old, nil)
		env.react.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)
		if err := react(env, student, "👍"); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("the cooldown is per stream and answers 429 without publishing", func(t *testing.T) {
		reactions(t, "👍")
		env := newWatchEnv(t)
		recent := model.StreamReaction{Model: gorm.Model{ID: 3, UpdatedAt: time.Now().Add(-2 * time.Second)}, Reaction: "👍", StreamID: 7, UserID: 20}
		env.react.EXPECT().GetByStreamAndUser(gomock.Any(), uint(7), uint(20)).Return(recent, nil)
		wantChatCode(t, react(env, student, "👍"), codes.ResourceExhausted)
		env.none(t)
	})

	t.Run("v1's cooldown across streams is gone: only this stream's reaction is consulted", func(t *testing.T) {
		reactions(t, "👍")
		env := newWatchEnv(t)
		// A reaction to another stream a second ago is not even looked up; the DAO call
		// that found it (GetLastReactionOfUser) has no expectation here and would fail.
		env.react.EXPECT().GetByStreamAndUser(gomock.Any(), uint(7), uint(20)).Return(model.StreamReaction{}, gorm.ErrRecordNotFound)
		env.react.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)
		if err := react(env, student, "👍"); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("refusals", func(t *testing.T) {
		reactions(t, "👍")
		env := newWatchEnv(t)
		wantChatCode(t, react(env, student, "💩"), codes.InvalidArgument)
		wantChatCode(t, react(env, student, ""), codes.InvalidArgument)
		env.course.Visibility = "public"
		wantChatCode(t, react(env, nil, "👍"), codes.Unauthenticated)
		env.course.Visibility = "enrolled"
		wantChatCode(t, react(env, outsider, "👍"), codes.PermissionDenied)
		env.course.Visibility = "loggedin"
		env.stream.Private = true
		wantChatCode(t, react(env, student, "👍"), codes.PermissionDenied)
		env.none(t)
	})

	t.Run("a storage failure is a 500 and publishes nothing", func(t *testing.T) {
		reactions(t, "👍")
		env := newWatchEnv(t)
		env.react.EXPECT().GetByStreamAndUser(gomock.Any(), uint(7), uint(20)).Return(model.StreamReaction{}, gorm.ErrRecordNotFound)
		env.react.EXPECT().Create(gomock.Any(), gomock.Any()).Return(errors.New("db down"))
		wantChatCode(t, react(env, student, "👍"), codes.Unknown)
		env.none(t)
	})
}

func TestReactionStats(t *testing.T) {
	reactions(t, "👍", "👎")
	now := time.Now()

	t.Run("a stream with no reactions is never ticked for", func(t *testing.T) {
		env := newWatchEnv(t)
		if env.api.tickReactionStats(context.Background(), now) {
			t.Error("nothing to tick for")
		}
		env.none(t)
		if env.api.reactions.running {
			t.Error("no ticker should be running")
		}
	})

	t.Run("a reacted-to stream is reported with shares until its window ends", func(t *testing.T) {
		env := newWatchEnv(t)
		env.api.touchReactionStats(7)
		env.api.touchReactionStats(8) // nobody watches stream 8
		env.react.EXPECT().GetByStreamWithinMinutes(gomock.Any(), uint(7), uint(15)).Return([]model.StreamReaction{
			{Reaction: "👍"}, {Reaction: "👍"}, {Reaction: "👍"}, {Reaction: "👎"},
		}, nil)

		if !env.api.tickReactionStats(context.Background(), time.Now()) {
			t.Fatal("streams are still active")
		}
		stats := env.next(t).GetStreamReactionStats()
		if stats.WindowSeconds != 900 || len(stats.Shares) != 2 ||
			stats.Shares[0].Reaction != "👍" || stats.Shares[0].Count != 3 || stats.Shares[0].Share != 0.75 ||
			stats.Shares[1].Count != 1 || stats.Shares[1].Share != 0.25 {
			t.Fatalf("stats = %v", stats)
		}
		env.none(t)
	})

	t.Run("after the window the stream gets one empty report and the ticker stops", func(t *testing.T) {
		env := newWatchEnv(t)
		env.api.touchReactionStats(7)
		if env.api.tickReactionStats(context.Background(), time.Now().Add(reactionWindow+time.Second)) {
			t.Error("nothing is left to tick for")
		}
		ev := env.next(t)
		if ev.GetStreamReactionStats() == nil || len(ev.GetStreamReactionStats().Shares) != 0 || ev.Audience != protobuf.RealtimeEvent_AUDIENCE_COURSE_ADMINS {
			t.Fatalf("event = %v", ev)
		}
		if env.api.reactions.running {
			t.Error("the ticker should have stopped")
		}
		// Ticking again does nothing.
		env.api.tickReactionStats(context.Background(), time.Now().Add(time.Hour))
		env.none(t)
	})
}

func enableAlerts(t *testing.T) {
	t.Helper()
	prev, prevURL := alertsConfigured, tools.Cfg.WebUrl
	alertsConfigured = func() bool { return true }
	tools.Cfg.WebUrl = "https://live.example"
	t.Cleanup(func() { alertsConfigured, tools.Cfg.WebUrl = prev, prevURL })
}

func TestReportStreamIssue(t *testing.T) {
	var sent []bot.AlertMessage
	var sendErr error
	setup := func(t *testing.T) *watchEnv {
		enableAlerts(t)
		env := newWatchEnv(t)
		sent, sendErr = nil, nil
		env.hall.CombIP, env.hall.CameraIP = "10.0.0.1/extron", "10.0.0.2"
		env.api.sendAlert = func(m bot.AlertMessage) error { sent = append(sent, m); return sendErr }
		return env
	}
	report := func(env *watchEnv, user *model.User, mutate ...func(*protobuf.ReportStreamIssueRequest)) error {
		req := &protobuf.ReportStreamIssueRequest{
			StreamId: 7, Comment: " no sound ", Name: "Max", Email: "max@example.org", Phone: "123",
			Categories: []protobuf.StreamIssueCategory{
				protobuf.StreamIssueCategory_STREAM_ISSUE_CATEGORY_AUDIO,
				protobuf.StreamIssueCategory_STREAM_ISSUE_CATEGORY_LIGHT,
				protobuf.StreamIssueCategory_STREAM_ISSUE_CATEGORY_OTHER,
			},
		}
		for _, m := range mutate {
			m(req)
		}
		_, err := env.api.ReportStreamIssue(asCaller(user), req)
		return err
	}

	t.Run("a student's report reaches the operators with the stream's details", func(t *testing.T) {
		env := setup(t)
		if err := report(env, student); err != nil {
			t.Fatalf("ReportStreamIssue: %v", err)
		}
		if len(sent) != 1 {
			t.Fatalf("sent %d alerts", len(sent))
		}
		m := sent[0]
		if m.Categories != "🔊 Audio · 💡 Light · Other" || m.Comment != "no sound" || m.Name != "Max" || m.Email != "max@example.org" || m.PhoneNumber != "123" ||
			m.StreamUrl != "https://live.example/w/gbs/7" || m.LectureHall != "MI HS1" || m.CourseName != "GBS" ||
			m.CombIP != "10.0.0.1/extron" || m.CameraIP != "10.0.0.2" || m.IsLecturer || m.User.ID != 20 || m.Stream.ID != 7 {
			t.Errorf("alert = %+v", m)
		}
	})

	t.Run("every category is labelled by its name", func(t *testing.T) {
		env := setup(t)
		for cat, want := range map[protobuf.StreamIssueCategory]string{
			protobuf.StreamIssueCategory_STREAM_ISSUE_CATEGORY_VIDEO:      "🎬 Video",
			protobuf.StreamIssueCategory_STREAM_ISSUE_CATEGORY_CAMERA:     "🎥 Camera",
			protobuf.StreamIssueCategory_STREAM_ISSUE_CATEGORY_MICROPHONE: "🎤 Microphone",
		} {
			sent = nil
			env.api.issues = issueLimiter{}
			if err := report(env, student, func(r *protobuf.ReportStreamIssueRequest) { r.Categories = []protobuf.StreamIssueCategory{cat} }); err != nil {
				t.Fatal(err)
			}
			if sent[0].Categories != want {
				t.Errorf("%v -> %q, want %q", cat, sent[0].Categories, want)
			}
		}
	})

	t.Run("a course administrator is marked as the lecturer", func(t *testing.T) {
		env := setup(t)
		if err := report(env, courseAdm); err != nil || !sent[0].IsLecturer {
			t.Fatalf("err %v sent %+v", err, sent)
		}
	})

	t.Run("a second report of the same stream within two minutes is a 429, another stream's is not", func(t *testing.T) {
		env := setup(t)
		if err := report(env, student); err != nil {
			t.Fatal(err)
		}
		wantChatCode(t, report(env, student), codes.ResourceExhausted)
		if err := report(env, outsider); err != nil {
			t.Errorf("another viewer: %v", err)
		}
		if len(sent) != 2 {
			t.Errorf("sent %d", len(sent))
		}
		if env.api.issues.allow(20, 8, time.Now()) != true {
			t.Error("another stream should be free")
		}
		if !env.api.issues.allow(20, 7, time.Now().Add(issueCooldown)) {
			t.Error("the slot should free after the cooldown")
		}
	})

	t.Run("a report that could not be sent does not use up the slot", func(t *testing.T) {
		env := setup(t)
		sendErr = errors.New("matrix down")
		wantChatCode(t, report(env, student), codes.Unknown)
		sendErr = nil
		if err := report(env, student); err != nil {
			t.Fatalf("retry: %v", err)
		}
	})

	t.Run("refusals send nothing", func(t *testing.T) {
		env := setup(t)
		env.course.Visibility = "public"
		wantChatCode(t, report(env, nil), codes.Unauthenticated)
		env.course.Visibility = "loggedin"
		wantChatCode(t, report(env, student, func(r *protobuf.ReportStreamIssueRequest) { r.Comment = "  " }), codes.InvalidArgument)
		wantChatCode(t, report(env, student, func(r *protobuf.ReportStreamIssueRequest) { r.Comment = strings.Repeat("x", 2001) }), codes.InvalidArgument)
		wantChatCode(t, report(env, student, func(r *protobuf.ReportStreamIssueRequest) { r.Name = strings.Repeat("x", 201) }), codes.InvalidArgument)
		wantChatCode(t, report(env, student, func(r *protobuf.ReportStreamIssueRequest) {
			r.Categories = []protobuf.StreamIssueCategory{protobuf.StreamIssueCategory_STREAM_ISSUE_CATEGORY_UNSPECIFIED}
		}), codes.InvalidArgument)
		env.course.Visibility = "enrolled"
		wantChatCode(t, report(env, outsider), codes.PermissionDenied)
		env.course.Visibility = "loggedin"
		env.stream.LectureHallID = 0
		wantChatCode(t, report(env, student), codes.InvalidArgument)
		env.stream.LectureHallID = 3
		alertsConfigured = func() bool { return false }
		wantChatCode(t, report(env, student), codes.Unavailable)
		if len(sent) != 0 {
			t.Errorf("sent %d alerts", len(sent))
		}
	})
}

func TestReportStreamSeek(t *testing.T) {
	seek := func(env *watchEnv, user *model.User, pos float64) error {
		_, err := env.api.ReportStreamSeek(asCaller(user), &protobuf.ReportStreamSeekRequest{StreamId: 7, Position: pos})
		return err
	}

	t.Run("counts a seek, for anonymous viewers of a public course too", func(t *testing.T) {
		env := newWatchEnv(t)
		env.course.Visibility = "public"
		env.seeks.EXPECT().Add("7", 12.5).Return(nil)
		env.seeks.EXPECT().Add("7", 3000.0).Return(nil)
		if err := seek(env, nil, 12.5); err != nil {
			t.Fatal(err)
		}
		if err := seek(env, nil, 3000); err != nil {
			t.Fatalf("the very end: %v", err)
		}
	})

	t.Run("refuses positions outside the recording and counts nothing", func(t *testing.T) {
		env := newWatchEnv(t)
		for _, pos := range []float64{3000.5, -1, nan()} {
			wantChatCode(t, seek(env, student, pos), codes.InvalidArgument)
		}
		env.stream.Duration.Valid = false
		wantChatCode(t, seek(env, student, 1), codes.InvalidArgument)
	})

	t.Run("checks access", func(t *testing.T) {
		env := newWatchEnv(t)
		wantChatCode(t, seek(env, nil, 1), codes.PermissionDenied)
		env.course.Visibility = "enrolled"
		wantChatCode(t, seek(env, outsider, 1), codes.PermissionDenied)
		env.course.Visibility = "public"
		env.stream.Private = true
		wantChatCode(t, seek(env, student, 1), codes.PermissionDenied)
	})

	t.Run("a storage failure is a 500", func(t *testing.T) {
		env := newWatchEnv(t)
		env.seeks.EXPECT().Add("7", 1.0).Return(errors.New("db down"))
		wantChatCode(t, seek(env, student, 1), codes.Unknown)
	})
}

func nan() float64 { var z float64; return z / z }

func TestGetStreamSeekHeatmap(t *testing.T) {
	chunks := func(n, hits int) []model.VideoSeekChunk {
		var out []model.VideoSeekChunk
		for i := 0; i < n; i++ {
			out = append(out, model.VideoSeekChunk{StreamID: 7, ChunkIndex: uint(i), Hits: uint(hits)})
		}
		return out
	}
	get := func(env *watchEnv, user *model.User) (*protobuf.GetStreamSeekHeatmapResponse, error) {
		return env.api.GetStreamSeekHeatmap(asCaller(user), &protobuf.GetStreamSeekHeatmapRequest{StreamId: 7})
	}

	t.Run("answers only the slices hit, once there is enough to show", func(t *testing.T) {
		env := newWatchEnv(t)
		data := append(chunks(60, 3), model.VideoSeekChunk{StreamID: 7, ChunkIndex: 100, Hits: 0})
		env.seeks.EXPECT().Get("7").Return(data, nil)
		res, err := get(env, student)
		if err != nil || res.BucketCount != 150 || len(res.Buckets) != 60 || res.Buckets[5].Index != 5 || res.Buckets[5].Hits != 3 {
			t.Fatalf("res = %v, err = %v", res, err)
		}
	})
	for name, data := range map[string][]model.VideoSeekChunk{
		"too few hits":   chunks(60, 2),  // 120 < 150
		"too few slices": chunks(49, 10), // 490 hits but 49 < 50 slices
		"nothing":        nil,
	} {
		t.Run("empty with "+name, func(t *testing.T) {
			env := newWatchEnv(t)
			env.seeks.EXPECT().Get("7").Return(data, nil)
			res, err := get(env, student)
			if err != nil || res.BucketCount != 150 || len(res.Buckets) != 0 {
				t.Fatalf("res = %v, err = %v", res, err)
			}
		})
	}
	t.Run("checks access", func(t *testing.T) {
		env := newWatchEnv(t)
		_, err := get(env, nil)
		wantChatCode(t, err, codes.PermissionDenied)
	})
}

func hit(prev, text, next string) meilisearch.Hit {
	return meilisearch.Hit{
		"timestamp": []byte(`1500`), "textPrev": []byte(`"` + prev + `"`),
		"text": []byte(`"` + text + `"`), "textNext": []byte(`"` + next + `"`),
	}
}

func TestSearchStreamSubtitles(t *testing.T) {
	setup := func(t *testing.T) (*watchEnv, *mock_tools.MockMeiliSearchInterface) {
		env := newWatchEnv(t)
		m := mock_tools.NewMockMeiliSearchInterface(gomock.NewController(t))
		env.api.subtitleSearch = m
		return env, m
	}
	search := func(env *watchEnv, user *model.User, q string, limit uint32) (*protobuf.SearchStreamSubtitlesResponse, error) {
		return env.api.SearchStreamSubtitles(asCaller(user), &protobuf.SearchStreamSubtitlesRequest{StreamId: 7, Query: q, Limit: limit})
	}

	t.Run("filters on the stream and returns the lines", func(t *testing.T) {
		env, m := setup(t)
		m.EXPECT().Search("hopfen", int64(10), 1, "", "", "streamID = 7").Return(&meilisearch.MultiSearchResponse{
			Results: []meilisearch.SearchResponse{{IndexUID: "SUBTITLES", Hits: []meilisearch.Hit{hit("a", "hopfen", "b")}}},
		})
		res, err := search(env, student, " hopfen ", 0)
		if err != nil || len(res.Hits) != 1 {
			t.Fatalf("res = %v, err = %v", res, err)
		}
		if h := res.Hits[0]; h.Timestamp != 1500 || h.TextPrev != "a" || h.Text != "hopfen" || h.TextNext != "b" {
			t.Errorf("hit = %v", h)
		}
	})

	t.Run("limits default to 10 and stop at 100", func(t *testing.T) {
		env, m := setup(t)
		m.EXPECT().Search("hopfen", int64(100), 1, "", "", "streamID = 7").Return(&meilisearch.MultiSearchResponse{})
		if res, err := search(env, student, "hopfen", 5000); err != nil || len(res.Hits) != 0 {
			t.Fatalf("res = %v, err = %v", res, err)
		}
	})

	t.Run("short queries are refused", func(t *testing.T) {
		env, _ := setup(t)
		_, err := search(env, student, " ab ", 0)
		wantChatCode(t, err, codes.InvalidArgument)
	})

	t.Run("an unreachable Meilisearch is a 503", func(t *testing.T) {
		env, m := setup(t)
		m.EXPECT().Search(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
		_, err := search(env, student, "hopfen", 0)
		wantChatCode(t, err, codes.Unavailable)

		env.api.subtitleSearch = nil
		_, err = search(env, student, "hopfen", 0)
		wantChatCode(t, err, codes.Unavailable)
	})

	t.Run("checks access first", func(t *testing.T) {
		env, _ := setup(t)
		_, err := search(env, nil, "hopfen", 0)
		wantChatCode(t, err, codes.PermissionDenied)
		env.course.Visibility = "enrolled"
		_, err = search(env, outsider, "hopfen", 0)
		wantChatCode(t, err, codes.PermissionDenied)
	})
}

type fakeEnder struct {
	calls   []bool
	streams []uint
	err     error
}

func (f *fakeEnder) EndStream(_ context.Context, id uint, discard bool) error {
	f.streams, f.calls = append(f.streams, id), append(f.calls, discard)
	return f.err
}

func TestEndStream(t *testing.T) {
	setup := func(t *testing.T) (*watchEnv, *fakeEnder) {
		env := newWatchEnv(t)
		env.stream.Recording, env.stream.LiveNow = false, true
		f := &fakeEnder{}
		env.api.streamEnder = f
		return env, f
	}
	end := func(env *watchEnv, course uint32, discard bool) error {
		_, err := env.api.EndStream(asCaller(courseAdm), &protobuf.EndStreamRequest{CourseId: course, StreamId: 7, Discard: discard})
		return err
	}

	t.Run("ends a live stream of the course, keeping or discarding the recording", func(t *testing.T) {
		env, f := setup(t)
		env.audits.EXPECT().Create(gomock.Any()).Return(nil).Times(2)
		if err := end(env, 1, false); err != nil {
			t.Fatal(err)
		}
		if err := end(env, 1, true); err != nil {
			t.Fatal(err)
		}
		if len(f.calls) != 2 || f.streams[0] != 7 || f.calls[0] || !f.calls[1] {
			t.Errorf("calls = %v on %v", f.calls, f.streams)
		}
		// Telling the viewers is the ender's job (the runner manager's live-state
		// notifier), so the handler adds no second event.
		env.none(t)
	})

	t.Run("another course's stream is not found and not ended", func(t *testing.T) {
		env, f := setup(t)
		wantChatCode(t, end(env, 2, false), codes.NotFound)
		if len(f.calls) != 0 {
			t.Error("ended a foreign stream")
		}
	})

	t.Run("a stream that is not live is a 400", func(t *testing.T) {
		env, f := setup(t)
		env.stream.LiveNow = false
		wantChatCode(t, end(env, 1, false), codes.InvalidArgument)
		if len(f.calls) != 0 {
			t.Error("ended a stream that is not live")
		}
	})

	t.Run("failures", func(t *testing.T) {
		env, f := setup(t)
		f.err = errors.New("runner unreachable")
		wantChatCode(t, end(env, 1, false), codes.Unknown)
		env.api.streamEnder = nil
		wantChatCode(t, end(env, 1, false), codes.Unavailable)
	})
}

func TestUpdateLecturePublishesTimeChanges(t *testing.T) {
	setup := func(t *testing.T) (*API, lectureMocks, <-chan *protobuf.RealtimeEvent) {
		api, m := newLectureAPI(t)
		hub := realtimehub.NewMemory(realtimehub.WithViewersInterval(time.Hour))
		api.hub = hub
		events, cancel := hub.Subscribe(7)
		t.Cleanup(cancel)
		return api, m, events
	}
	nextEvent := func(t *testing.T, events <-chan *protobuf.RealtimeEvent) *protobuf.RealtimeEvent {
		for {
			select {
			case ev := <-events:
				if ev.GetViewers() == nil {
					return ev
				}
			case <-time.After(100 * time.Millisecond):
				return nil
			}
		}
	}

	t.Run("new times reach the viewers", func(t *testing.T) {
		api, m, events := setup(t)
		m.streams.EXPECT().GetStreamByID(gomock.Any(), "7").Return(courseOneLecture(), nil)
		m.streams.EXPECT().UpdateStream(gomock.Any()).Return(nil)
		start := lectureStart.Add(time.Hour)
		if _, err := api.UpdateLecture(asCaller(lectureAdmin), &protobuf.UpdateLectureRequest{
			CourseId: 1, StreamId: 7, Start: timestamppb.New(start), End: timestamppb.New(start.Add(time.Hour)),
		}); err != nil {
			t.Fatal(err)
		}
		ev := nextEvent(t, events)
		tc := ev.GetTimeChanged()
		if tc == nil || !tc.Start.AsTime().Equal(start) || !tc.End.AsTime().Equal(start.Add(time.Hour)) || ev.Audience != protobuf.RealtimeEvent_AUDIENCE_ALL {
			t.Fatalf("event = %v", ev)
		}
	})

	t.Run("times that did not change and edits to other fields publish none", func(t *testing.T) {
		api, m, events := setup(t)
		m.streams.EXPECT().GetStreamByID(gomock.Any(), "7").Return(courseOneLecture(), nil).Times(2)
		m.streams.EXPECT().UpdateStream(gomock.Any()).Return(nil).Times(2)
		if _, err := api.UpdateLecture(asCaller(lectureAdmin), &protobuf.UpdateLectureRequest{
			CourseId: 1, StreamId: 7, Start: timestamppb.New(lectureStart), End: timestamppb.New(lectureEnd),
		}); err != nil {
			t.Fatal(err)
		}
		if _, err := api.UpdateLecture(asCaller(lectureAdmin), &protobuf.UpdateLectureRequest{CourseId: 1, StreamId: 7, ChatEnabled: new(bool)}); err != nil {
			t.Fatal(err)
		}
		if ev := nextEvent(t, events); ev != nil {
			t.Fatalf("published %v", ev)
		}
	})

	t.Run("the series time update tells the lecture's own viewers", func(t *testing.T) {
		api, m, events := setup(t)
		m.streams.EXPECT().GetStreamByID(gomock.Any(), "7").Return(courseOneLecture(), nil)
		start, end := lectureStart.Add(time.Hour), lectureEnd.Add(time.Hour)
		m.streams.EXPECT().UpdateCourseLectureSeriesTime(uint(1), uint(7), "s1", start, end).Return(nil)
		if _, err := api.UpdateLectureSeriesTime(asCaller(lectureAdmin), &protobuf.UpdateLectureSeriesTimeRequest{
			CourseId: 1, StreamId: 7, Start: timestamppb.New(start), End: timestamppb.New(end),
		}); err != nil {
			t.Fatal(err)
		}
		if ev := nextEvent(t, events); ev.GetTimeChanged() == nil {
			t.Fatalf("event = %v", ev)
		}
	})
}
