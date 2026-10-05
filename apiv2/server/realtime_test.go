package apiv2

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"go.uber.org/mock/gomock"
	"google.golang.org/protobuf/encoding/protojson"
	"gorm.io/gorm"

	protobuf "github.com/TUM-Dev/gocast/apiv2/protobuf/server"
	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/mock_dao"
	"github.com/TUM-Dev/gocast/model"
	"github.com/TUM-Dev/gocast/pkg/realtimehub"
)

// realtimeServer serves the socket for stream 7 of course 5 and nothing else.
func realtimeServer(t *testing.T, visibility string) (*httptest.Server, *realtimehub.Memory) {
	t.Helper()
	ctrl := gomock.NewController(t)

	streams := mock_dao.NewMockStreamsDao(ctrl)
	streams.EXPECT().GetStreamByID(gomock.Any(), "7").
		Return(model.Stream{Model: gorm.Model{ID: 7}, CourseID: 5}, nil).AnyTimes()
	streams.EXPECT().GetStreamByID(gomock.Any(), gomock.Not("7")).
		Return(model.Stream{}, gorm.ErrRecordNotFound).AnyTimes()
	courses := mock_dao.NewMockCoursesDao(ctrl)
	courses.EXPECT().GetCourseById(gomock.Any(), uint(5)).
		Return(model.Course{Model: gorm.Model{ID: 5}, Visibility: visibility}, nil).AnyTimes()

	hub := realtimehub.NewMemory(realtimehub.WithViewersInterval(time.Hour))
	api := &API{
		dao: dao.DaoWrapper{StreamsDao: streams, CoursesDao: courses},
		log: slog.Default(),
		hub: hub,
	}

	srv := httptest.NewServer(http.HandlerFunc(api.serveRealtime))
	t.Cleanup(srv.Close)
	return srv, hub
}

func dialRealtime(t *testing.T, srv *httptest.Server, query string) *websocket.Conn {
	t.Helper()
	url := "ws" + strings.TrimPrefix(srv.URL, "http") + "/api/v2/realtime?" + query
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	return conn
}

// readEvent returns the next frame as an event, failing on anything else.
func readEvent(t *testing.T, conn *websocket.Conn) *protobuf.RealtimeEvent {
	t.Helper()
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, frame, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var ev protobuf.RealtimeEvent
	if err := protojson.Unmarshal(frame, &ev); err != nil {
		t.Fatalf("frame %s is not a RealtimeEvent: %v", frame, err)
	}
	return &ev
}

// closeCode reads until the server closes the socket and returns its close code.
func closeCode(t *testing.T, conn *websocket.Conn) int {
	t.Helper()
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	for {
		_, _, err := conn.ReadMessage()
		if err == nil {
			continue
		}
		var closeErr *websocket.CloseError
		if !errors.As(err, &closeErr) {
			t.Fatalf("socket ended without a close frame: %v", err)
		}
		return closeErr.Code
	}
}

func TestRealtimeSocket(t *testing.T) {
	t.Run("an anonymous viewer hears the count, then the stream's events", func(t *testing.T) {
		srv, hub := realtimeServer(t, "public")
		conn := dialRealtime(t, srv, "stream=7")
		if err := conn.WriteMessage(websocket.TextMessage, []byte(`{}`)); err != nil {
			t.Fatal(err)
		}

		if got := readEvent(t, conn).GetViewers().GetViewers(); got != 1 {
			t.Fatalf("first frame's viewers = %d, want 1", got)
		}

		// Meant for the moderators, so this socket must skip it...
		moderated := realtimehub.TitleEvent(7, "secret")
		moderated.Audience = protobuf.RealtimeEvent_AUDIENCE_COURSE_ADMINS
		hub.Publish(context.Background(), 7, moderated)
		// ...and this one, whose audience nobody set.
		unset := realtimehub.TitleEvent(7, "unset")
		unset.Audience = protobuf.RealtimeEvent_AUDIENCE_UNSPECIFIED
		hub.Publish(context.Background(), 7, unset)
		hub.Publish(context.Background(), 7, realtimehub.TitleEvent(7, "Hopfen"))

		for {
			ev := readEvent(t, conn)
			if ev.GetViewers() != nil {
				continue // the hub's own count, on its own schedule
			}
			if got := ev.GetTitle().GetTitle(); got != "Hopfen" {
				t.Fatalf("title = %q, want Hopfen: the socket let through an event not meant for it", got)
			}
			break
		}
	})

	t.Run("refuses a stream that is not a number", func(t *testing.T) {
		srv, _ := realtimeServer(t, "public")
		if code := closeCode(t, dialRealtime(t, srv, "stream=abc")); code != 4400 {
			t.Errorf("close code = %d, want 4400", code)
		}
	})

	t.Run("refuses a stream that does not exist", func(t *testing.T) {
		srv, _ := realtimeServer(t, "public")
		conn := dialRealtime(t, srv, "stream=8")
		_ = conn.WriteMessage(websocket.TextMessage, []byte(`{}`))
		if code := closeCode(t, conn); code != 4404 {
			t.Errorf("close code = %d, want 4404", code)
		}
	})

	t.Run("refuses an anonymous viewer of a course for signed-in users", func(t *testing.T) {
		srv, _ := realtimeServer(t, "loggedin")
		conn := dialRealtime(t, srv, "stream=7")
		_ = conn.WriteMessage(websocket.TextMessage, []byte(`{}`))
		if code := closeCode(t, conn); code != 4403 {
			t.Errorf("close code = %d, want 4403", code)
		}
	})

	t.Run("refuses a token it cannot verify", func(t *testing.T) {
		srv, _ := realtimeServer(t, "public")
		conn := dialRealtime(t, srv, "stream=7")
		_ = conn.WriteMessage(websocket.TextMessage, []byte(`{"token":"not-a-jwt"}`))
		if code := closeCode(t, conn); code != 4401 {
			t.Errorf("close code = %d, want 4401", code)
		}
	})

	t.Run("refuses a first frame that is not a credentials frame", func(t *testing.T) {
		srv, _ := realtimeServer(t, "public")
		conn := dialRealtime(t, srv, "stream=7")
		_ = conn.WriteMessage(websocket.TextMessage, []byte(`hello`))
		if code := closeCode(t, conn); code != 4401 {
			t.Errorf("close code = %d, want 4401", code)
		}
	})

	t.Run("gives up on a client that sends no credentials", func(t *testing.T) {
		old := realtimeAuthTimeout
		realtimeAuthTimeout = 50 * time.Millisecond
		t.Cleanup(func() { realtimeAuthTimeout = old })

		srv, _ := realtimeServer(t, "public")
		if code := closeCode(t, dialRealtime(t, srv, "stream=7")); code != 4401 {
			t.Errorf("close code = %d, want 4401", code)
		}
	})

	t.Run("leaving gives up the subscription", func(t *testing.T) {
		srv, hub := realtimeServer(t, "public")
		conn := dialRealtime(t, srv, "stream=7")
		_ = conn.WriteMessage(websocket.TextMessage, []byte(`{}`))
		readEvent(t, conn)
		conn.Close()

		deadline := time.Now().Add(2 * time.Second)
		for hub.Viewers(7) != 0 {
			if time.Now().After(deadline) {
				t.Fatalf("still %d viewers after the socket closed", hub.Viewers(7))
			}
			time.Sleep(10 * time.Millisecond)
		}
	})
}

func TestRealtimeAudience(t *testing.T) {
	event := func(audience protobuf.RealtimeEvent_Audience, user uint32) *protobuf.RealtimeEvent {
		return &protobuf.RealtimeEvent{Audience: audience, AudienceUserId: user}
	}
	anonymous := realtimeViewer{}
	student := realtimeViewer{userID: 3}
	admin := realtimeViewer{userID: 9, isAdmin: true}

	cases := []struct {
		name   string
		event  *protobuf.RealtimeEvent
		viewer realtimeViewer
		want   bool
	}{
		{"everyone hears ALL", event(protobuf.RealtimeEvent_AUDIENCE_ALL, 0), anonymous, true},
		{"nobody hears UNSPECIFIED", event(protobuf.RealtimeEvent_AUDIENCE_UNSPECIFIED, 0), admin, false},
		{"admins hear COURSE_ADMINS", event(protobuf.RealtimeEvent_AUDIENCE_COURSE_ADMINS, 0), admin, true},
		{"students do not", event(protobuf.RealtimeEvent_AUDIENCE_COURSE_ADMINS, 0), student, false},
		{"the author of a moderated message does", event(protobuf.RealtimeEvent_AUDIENCE_COURSE_ADMINS, 3), student, true},
		{"USER reaches its user", event(protobuf.RealtimeEvent_AUDIENCE_USER, 3), student, true},
		{"and not an admin", event(protobuf.RealtimeEvent_AUDIENCE_USER, 3), admin, false},
		{"user 0 is nobody", event(protobuf.RealtimeEvent_AUDIENCE_USER, 0), anonymous, false},
	}
	for _, c := range cases {
		if got := c.viewer.receives(c.event); got != c.want {
			t.Errorf("%s: receives = %v, want %v", c.name, got, c.want)
		}
	}
}

// The rule v1 applies when a viewer leaves the chat channel.
func TestCountVodView(t *testing.T) {
	recording := model.Stream{Model: gorm.Model{ID: 7}, Recording: true}

	cases := []struct {
		name   string
		stream model.Stream
		stayed time.Duration
		counts bool
	}{
		{"a recording watched for six minutes", recording, 6 * time.Minute, true},
		{"a recording watched for four", recording, 4 * time.Minute, false},
		{"a live stream watched for an hour", model.Stream{Model: gorm.Model{ID: 7}}, time.Hour, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			streams := mock_dao.NewMockStreamsDao(gomock.NewController(t))
			if c.counts {
				streams.EXPECT().AddVodView("7").Return(nil)
			}
			api := &API{dao: dao.DaoWrapper{StreamsDao: streams}, log: slog.Default()}

			api.countVodView(c.stream, time.Now().Add(-c.stayed))
		})
	}
}
