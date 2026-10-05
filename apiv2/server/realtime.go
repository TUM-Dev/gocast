package apiv2

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/websocket"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"

	protobuf "github.com/TUM-Dev/gocast/apiv2/protobuf/server"
	"github.com/TUM-Dev/gocast/model"
	"github.com/TUM-Dev/gocast/pkg/realtimehub"
)

// WithRealtimeHub shares the hub the realtime socket reads from with the producers
// that publish to it from outside this package.
func WithRealtimeHub(hub realtimehub.Hub) Option {
	return func(a *API) { a.hub = hub }
}

// publish sends an event to the stream's realtime subscribers. An API built without
// New, as the handler tests build theirs, has no hub and publishes nothing.
func (a *API) publish(ctx context.Context, streamID uint, ev *protobuf.RealtimeEvent) {
	if a.hub != nil {
		a.hub.Publish(ctx, streamID, ev)
	}
}

// realtimeAuthTimeout is how long a socket without a session cookie has to send its
// credentials frame. A variable only so the test of it does not take five seconds.
var realtimeAuthTimeout = 5 * time.Second

const (
	realtimeWriteTimeout = 10 * time.Second
	realtimePingInterval = 30 * time.Second
	// A client that has not answered two pings is gone.
	realtimePongTimeout = 2*realtimePingInterval + realtimeWriteTimeout

	// The only frame a client sends is its credentials, and a JWT fits in this.
	realtimeMaxClientFrame = 8 << 10

	// A socket open this long on a recording counts as a view of it, as v1 counts one
	// when a viewer leaves the chat channel.
	realtimeVodViewAfter = 5 * time.Minute

	// Close codes 4000 + the HTTP status the same failure gets over REST, so a client
	// can tell "sign in" from "not allowed" from "no such stream". Browsers hide the
	// handshake's own status, which is why the upgrade succeeds and the socket is
	// closed with one of these instead.
	closeCodeBase = 4000
)

// realtimeUpgrader keeps gorilla's default origin check: a socket authenticated by
// the session cookie must not be openable from another site's page.
var realtimeUpgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 4096,
}

// realtimeHello is the frame a client without a session cookie sends first: its
// bearer token, or nothing to watch anonymously.
type realtimeHello struct {
	Token string `json:"token"`
}

// RealtimePath is the realtime socket, GET /api/v2/realtime?stream=<id>. It pushes
// the stream's RealtimeEvents, protojson encoded, one per text frame.
//
// Served beside the gateway rather than through it, since gRPC-gateway cannot serve a
// websocket and MaxConnectionAge rules out gRPC streaming. It must be excluded from
// the gzip middleware, which would wrap the connection the upgrade has to hijack.
const RealtimePath = "/api/v2/realtime"

func (a *API) serveRealtime(w http.ResponseWriter, r *http.Request) {
	conn, err := realtimeUpgrader.Upgrade(w, r, nil)
	if err != nil {
		// The upgrader has already answered with an HTTP error.
		return
	}
	defer conn.Close()

	streamID, err := strconv.ParseUint(r.URL.Query().Get("stream"), 10, 32)
	if err != nil || streamID == 0 {
		closeRealtime(conn, closeCodeBase+http.StatusBadRequest, "stream must be a stream id")
		return
	}

	md, err := realtimeCredentials(conn, r)
	if err != nil {
		closeRealtime(conn, closeCodeBase+http.StatusUnauthorized, err.Error())
		return
	}

	// The same resolution and the same eligibility rule as every stream RPC, so the
	// socket cannot be a way around them.
	ctx := metadata.NewIncomingContext(r.Context(), md)
	user, err := a.resolveCurrent(ctx)
	ctx = context.WithValue(ctx, callerKey{}, &caller{user: user, err: err})
	user, stream, course, err := a.authorizeUserForStreamCourse(ctx, &protobuf.ListChatMessagesRequest{StreamId: uint32(streamID)})
	if err != nil {
		closeRealtime(conn, closeCodeBase+runtime.HTTPStatusFromCode(status.Code(err)), status.Convert(err).Message())
		return
	}

	v := realtimeViewer{isAdmin: user.CanAdminister(course)}
	if user != nil {
		v.userID = uint32(user.ID)
	}

	events, unsubscribe := a.hub.Subscribe(stream.ID)
	joined := time.Now()
	defer func() {
		unsubscribe()
		a.countVodView(stream, joined)
	}()

	// The count as of joining, straight away: the hub's own ViewersEvent is throttled.
	if err := a.writeRealtime(conn, realtimehub.ViewersEvent(stream.ID, a.hub.Viewers(stream.ID))); err != nil {
		return
	}

	a.pumpRealtime(conn, events, v)
}

// realtimeCredentials returns what to authenticate the socket with, in the metadata
// shape resolveCurrent reads. The session cookie is used when present; otherwise the
// client's first frame must arrive within realtimeAuthTimeout and carry a bearer
// token, or no token for an anonymous viewer. A first frame sent despite a cookie is
// read and ignored with the rest.
func realtimeCredentials(conn *websocket.Conn, r *http.Request) (metadata.MD, error) {
	md := metadata.MD{}

	cookies := r.Header.Get("Cookie")
	if _, err := extractTokenFromCookie(cookies); err == nil {
		md.Set("grpcgateway-cookie", cookies)
		return md, nil
	}

	conn.SetReadLimit(realtimeMaxClientFrame)
	_ = conn.SetReadDeadline(time.Now().Add(realtimeAuthTimeout))
	kind, frame, err := conn.ReadMessage()
	if err != nil {
		return nil, errors.New(`send {"token":"..."}, or {} to watch anonymously, within 5 seconds`)
	}

	var hello realtimeHello
	if kind != websocket.TextMessage || json.Unmarshal(frame, &hello) != nil {
		return nil, errors.New(`the first frame must be {"token":"..."} or {}`)
	}

	if hello.Token != "" {
		md.Set("authorization", "Bearer "+hello.Token)
	}

	return md, nil
}

// realtimeViewer is who a socket belongs to, for filtering events by audience.
type realtimeViewer struct {
	userID  uint32 // 0 when anonymous
	isAdmin bool
}

// receives reports whether the event's audience includes the viewer.
func (v realtimeViewer) receives(ev *protobuf.RealtimeEvent) bool {
	addressed := v.userID != 0 && ev.GetAudienceUserId() == v.userID

	switch ev.GetAudience() {
	case protobuf.RealtimeEvent_AUDIENCE_ALL:
		return true
	case protobuf.RealtimeEvent_AUDIENCE_COURSE_ADMINS:
		return v.isAdmin || addressed
	case protobuf.RealtimeEvent_AUDIENCE_USER:
		return addressed
	default:
		// Unspecified: fail closed rather than broadcast what was meant for few.
		return false
	}
}

// pumpRealtime forwards events until the client leaves or the hub drops it. The
// reader goroutine exists only to notice the client leaving and to answer pings;
// anything the client sends is discarded.
func (a *API) pumpRealtime(conn *websocket.Conn, events <-chan *protobuf.RealtimeEvent, v realtimeViewer) {
	gone := make(chan struct{})
	go func() {
		defer close(gone)
		conn.SetReadLimit(realtimeMaxClientFrame)
		_ = conn.SetReadDeadline(time.Now().Add(realtimePongTimeout))
		conn.SetPongHandler(func(string) error {
			return conn.SetReadDeadline(time.Now().Add(realtimePongTimeout))
		})
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}()

	ping := time.NewTicker(realtimePingInterval)
	defer ping.Stop()

	for {
		select {
		case <-gone:
			return

		case ev, ok := <-events:
			if !ok {
				// Dropped for falling behind; the ResyncEvent went out just before.
				// Closing makes the client reconnect, and it re-fetches on reconnect.
				closeRealtime(conn, websocket.CloseTryAgainLater, "fell behind")
				return
			}
			if !v.receives(ev) {
				continue
			}
			if err := a.writeRealtime(conn, ev); err != nil {
				return
			}

		case <-ping.C:
			if err := conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(realtimeWriteTimeout)); err != nil {
				return
			}
		}
	}
}

func (a *API) writeRealtime(conn *websocket.Conn, ev *protobuf.RealtimeEvent) error {
	frame, err := protojson.Marshal(ev)
	if err != nil {
		a.log.Error("can't encode realtime event", "err", err)
		return nil // one bad event is not a reason to drop the socket
	}

	_ = conn.SetWriteDeadline(time.Now().Add(realtimeWriteTimeout))
	return conn.WriteMessage(websocket.TextMessage, frame)
}

// countVodView records a view of a recording watched for long enough, the rule and
// the DAO call v1 applies when a viewer leaves the chat channel.
func (a *API) countVodView(stream model.Stream, joined time.Time) {
	if !stream.Recording || time.Since(joined) < realtimeVodViewAfter {
		return
	}
	if err := a.dao.AddVodView(strconv.FormatUint(uint64(stream.ID), 10)); err != nil {
		a.log.Error("can't save vod view", "err", err, "stream", stream.ID)
	}
}

// closeRealtime sends a close frame and gives up on the connection; the caller's
// deferred Close tears it down.
func closeRealtime(conn *websocket.Conn, code int, reason string) {
	// A close frame's reason is limited to 123 bytes.
	if len(reason) > 123 {
		reason = reason[:123]
	}
	_ = conn.WriteControl(
		websocket.CloseMessage,
		websocket.FormatCloseMessage(code, reason),
		time.Now().Add(realtimeWriteTimeout),
	)
}
