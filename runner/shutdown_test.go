package runner

import (
	"context"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sethvargo/go-retry"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	"github.com/tum-dev/gocast/runner/pkg/actions"
	"github.com/tum-dev/gocast/runner/pkg/metrics"
	"github.com/tum-dev/gocast/runner/protobuf"
)

// blockingManager is a manager whose Notify does not answer until it is released.
type blockingManager struct {
	protobuf.RunnerManagerServiceClient
	arrived chan struct{}
	release chan struct{}
}

func (m *blockingManager) Notify(ctx context.Context, _ *protobuf.Notification, _ ...grpc.CallOption) (*protobuf.NotificationResponse, error) {
	m.arrived <- struct{}{}
	select {
	case <-m.release:
		return &protobuf.NotificationResponse{}, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// waitIdleAsync runs WaitIdle and returns a channel that is closed once it returned.
func waitIdleAsync(r *Runner) <-chan struct{} {
	done := make(chan struct{})
	go func() { r.WaitIdle(); close(done) }()
	return done
}

func assertStillWaiting(t *testing.T, done <-chan struct{}, why string) {
	t.Helper()
	select {
	case <-done:
		t.Fatalf("WaitIdle returned: %s", why)
	case <-time.After(300 * time.Millisecond):
	}
}

func assertIdle(t *testing.T, done <-chan struct{}) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("WaitIdle did not return once everything was done")
	}
}

// The notification of a job's result is handed to handleNotifications, which delivers it
// in a goroutine of its own. The job is over by then, but the runner must not exit before
// the manager has the VODReady: nobody else would ever tell it.
func TestWaitIdleWaitsForTheLastJobsNotification(t *testing.T) {
	r := newTestRunner()
	r.notifications = make(chan *protobuf.Notification) // as in production: unbuffered
	conn, err := grpc.NewClient("127.0.0.1:1", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	mgr := &blockingManager{arrived: make(chan struct{}, 8), release: make(chan struct{})}
	r.managerConn, r.managerClient = conn, mgr

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go r.handleNotifications(ctx)

	sent := func(_ context.Context, _ *slog.Logger, n chan *protobuf.Notification, _ map[string]any, _ *metrics.Broker) error {
		n <- &protobuf.Notification{Data: &protobuf.Notification_VodReady{VodReady: &protobuf.VODReadyNotification{}}}
		return nil
	}
	r.RunAction([]actions.Action{sent}, nil, nil, r.log)

	select {
	case <-mgr.arrived:
	case <-time.After(3 * time.Second):
		t.Fatal("the notification never reached the manager")
	}
	done := waitIdleAsync(r)
	assertStillWaiting(t, done, "the VODReady was still being delivered")

	close(mgr.release)
	assertIdle(t, done)
}

func TestWaitIdleReturnsWithoutPendingWork(t *testing.T) {
	r := newTestRunner()
	assertIdle(t, waitIdleAsync(r))
}

// The body of an upload arrives before RunAction creates its job. A runner told to shut
// down in between has no job, but it must not exit and cut the upload off.
func TestWaitIdleWaitsForAnUploadThatIsStillArriving(t *testing.T) {
	prev := importActions
	importActions = []actions.Action{func(context.Context, *slog.Logger, chan *protobuf.Notification, map[string]any, *metrics.Broker) error {
		return nil
	}}
	t.Cleanup(func() { importActions = prev })

	r := newUploadTestRunner(t)
	token, err := r.uploads.create(1, "STREAM_VERSION_COMB", false)
	if err != nil {
		t.Fatal(err)
	}

	pr, pw := io.Pipe()
	mw := multipart.NewWriter(pw)
	rest := make(chan struct{})
	go func() {
		fw, _ := mw.CreateFormFile("file", "lecture.mp4")
		_, _ = fw.Write([]byte("first half"))
		<-rest // the uploader is slow
		_, _ = fw.Write([]byte("second half"))
		_ = mw.Close()
		_ = pw.Close()
	}()

	req := httptest.NewRequest(http.MethodPost, "/upload/"+token, pr)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	rec := httptest.NewRecorder()
	handled := make(chan struct{})
	go func() { r.uploadHandler().ServeHTTP(rec, req); close(handled) }()

	deadline := time.Now().Add(3 * time.Second)
	for busy, _ := r.counts(); busy == 0; busy, _ = r.counts() {
		if time.Now().After(deadline) {
			t.Fatal("the upload did not hold the runner while it was arriving")
		}
		time.Sleep(5 * time.Millisecond)
	}
	if len(r.jobs) != 0 {
		t.Fatal("test setup: the import job already exists")
	}
	done := waitIdleAsync(r)
	assertStillWaiting(t, done, "the upload body was still arriving")

	close(rest)
	<-handled
	if rec.Code != http.StatusOK {
		t.Fatalf("upload answered %d: %s", rec.Code, rec.Body.String())
	}
	assertIdle(t, done)
}

// failingManager answers every Notify with err and counts the calls.
type failingManager struct {
	protobuf.RunnerManagerServiceClient
	err   error
	calls atomic.Int32
}

func (m *failingManager) Notify(context.Context, *protobuf.Notification, ...grpc.CallOption) (*protobuf.NotificationResponse, error) {
	m.calls.Add(1)
	return nil, m.err
}

func newManagerTestRunner(t *testing.T, mgr protobuf.RunnerManagerServiceClient) *Runner {
	t.Helper()
	r := newTestRunner()
	conn, err := grpc.NewClient("127.0.0.1:1", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	r.managerConn, r.managerClient = conn, mgr
	return r
}

func TestSendNotificationRetriesOnlyWhatMayChange(t *testing.T) {
	n := &protobuf.Notification{Data: &protobuf.Notification_VodReady{VodReady: &protobuf.VODReadyNotification{}}}
	for code, wantCalls := range map[codes.Code]int32{
		// Unavailable is retried too, but it drops the client (see shouldReconnect), so the
		// fake is gone after the first call.
		codes.Internal:         3,
		codes.DeadlineExceeded: 3,
		codes.NotFound:         1, // the manager refuses it and will keep doing so
		codes.InvalidArgument:  1,
	} {
		mgr := &failingManager{err: status.Error(code, "x")}
		r := newManagerTestRunner(t, mgr)
		b := retry.WithMaxRetries(2, retry.NewConstant(time.Millisecond))
		_ = retry.Do(context.Background(), b, r.sendNotification(n))
		if got := mgr.calls.Load(); got != wantCalls {
			t.Errorf("%v: Notify called %d times, want %d", code, got, wantCalls)
		}
	}
}

// A notification that is refused for good must not keep a finished runner alive.
func TestWaitIdleDoesNotWaitForARejectedNotification(t *testing.T) {
	r := newManagerTestRunner(t, &failingManager{err: status.Error(codes.NotFound, "no such stream")})
	r.notifications = make(chan *protobuf.Notification)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go r.handleNotifications(ctx)

	r.notifications <- &protobuf.Notification{Data: &protobuf.Notification_StreamEnd{StreamEnd: &protobuf.StreamEndNotification{}}}
	assertIdle(t, waitIdleAsync(r))
}

// Notifications the manager does not accept for a long time are given up on after the
// grace, but not while a job still runs: that one is waited for as long as it takes.
func TestWaitIdleGivesUpOnUndeliveredNotificationsAfterTheGrace(t *testing.T) {
	r := newManagerTestRunner(t, &failingManager{err: status.Error(codes.Unavailable, "manager is restarting")})
	r.notifications = make(chan *protobuf.Notification)
	r.notifyGrace = 400 * time.Millisecond
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go r.handleNotifications(ctx)

	r.notifications <- &protobuf.Notification{Data: &protobuf.Notification_VodReady{VodReady: &protobuf.VODReadyNotification{}}}
	jobDone := r.hold() // a job that is still running
	done := waitIdleAsync(r)

	time.Sleep(2 * r.notifyGrace)
	assertStillWaiting(t, done, "a job was still running")

	jobDone()
	start := time.Now()
	assertIdle(t, done)
	if waited := time.Since(start); waited < r.notifyGrace/2 {
		t.Errorf("gave up after %v, the grace only starts once the job is done", waited)
	}
}

// A heartbeat that was built before the drain and is still on its way after the drain
// heartbeat would tell the manager that the runner accepts jobs again: it stores whatever
// arrives last.
func TestStaleHeartbeatIsNotSentOnceDraining(t *testing.T) {
	mgr := &failingManager{err: nil}
	r := newManagerTestRunner(t, mgr)
	stale := r.heartbeat() // built before the signal
	r.Drain(context.Background())
	drain := <-r.notifications

	if err := r.sendNotification(stale)(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := mgr.calls.Load(); got != 0 {
		t.Fatalf("a heartbeat without draining was sent %d times after the drain started", got)
	}
	if err := r.sendNotification(drain)(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := mgr.calls.Load(); got != 1 {
		t.Errorf("the drain heartbeat was sent %d times, want 1", got)
	}
}

// The next heartbeat is seconds away, a late retry only delivers outdated state.
func TestHeartbeatIsNotRetried(t *testing.T) {
	if n := drainBackoff(notificationBackoff(newTestRunner().heartbeat()), 100); n != 0 {
		t.Errorf("a heartbeat is retried %d times, want 0", n)
	}
}
