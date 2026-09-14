package runner

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/tum-dev/gocast/runner/pkg/actions"
	"github.com/tum-dev/gocast/runner/pkg/metrics"
	"github.com/tum-dev/gocast/runner/pkg/ptr"
	"github.com/tum-dev/gocast/runner/protobuf"
)

// newTestRunner builds a Runner with just the state RunAction and RequestStreamEnd touch.
// JobCount is buffered because RunAction sends on it synchronously; in production Run
// drains it from the heartbeat loop.
func newTestRunner() *Runner {
	return &Runner{
		log:           slog.New(slog.NewTextHandler(io.Discard, nil)),
		JobCount:      make(chan int, 64),
		jobs:          make(map[string]context.CancelFunc),
		discard:       make(map[string]bool),
		notifications: make(chan *protobuf.Notification, 64),
	}
}

// recorder collects the names of the actions that ran, in order.
type recorder struct {
	mu    sync.Mutex
	calls []string
}

func (r *recorder) record(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, name)
}

func (r *recorder) get() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.calls...)
}

// action returns an action that records it ran and then returns errs[n] on its n-th call.
func (r *recorder) action(name string, errs ...error) actions.Action {
	var calls int
	return func(_ context.Context, _ *slog.Logger, _ chan *protobuf.Notification, _ map[string]any, _ *metrics.Broker) error {
		r.record(name)
		defer func() { calls++ }()
		if calls < len(errs) {
			return errs[calls]
		}
		return nil
	}
}

// waitForJob blocks until RunAction's goroutine removed the job from r.jobs.
func waitForJob(t *testing.T, r *Runner, job string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		r.jobsMu.Lock()
		_, running := r.jobs[job]
		r.jobsMu.Unlock()
		if !running {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("job %s did not finish in time", job)
}

func assertCalls(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("actions ran: got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("actions ran: got %v, want %v", got, want)
		}
	}
}

func TestRunActionRunsVoDActionsWhenStreamEndsOnItsOwn(t *testing.T) {
	r := newTestRunner()
	rec := &recorder{}

	job := r.RunAction(
		[]actions.Action{rec.action("stream"), rec.action("streamEnd")},
		[]actions.Action{rec.action("mkVod"), rec.action("checkVod"), rec.action("mkThumb")},
		map[string]any{}, r.log,
	)
	waitForJob(t, r, job)

	assertCalls(t, rec.get(), []string{"stream", "streamEnd", "mkVod", "checkVod", "mkThumb"})
}

// endJob ends the job the way the gRPC handler does and fails the test if the runner
// does not know it.
func endJob(t *testing.T, r *Runner, job string, discard bool) {
	t.Helper()
	resp, err := r.RequestStreamEnd(context.Background(), &protobuf.StreamEndRequest{
		JobId:      ptr.Take(job),
		DiscardVod: ptr.Take(discard),
	})
	if err != nil {
		t.Fatalf("RequestStreamEnd: %v", err)
	}
	// (nil, nil) is an internal error in gRPC, the handler has to return a message.
	if resp == nil {
		t.Fatal("RequestStreamEnd returned a nil response for a known job")
	}
}

func TestRunActionSkipsVoDActionsWhenDiscarded(t *testing.T) {
	r := newTestRunner()
	rec := &recorder{}

	// blocks the first action until the test ended the stream, so that the discard flag
	// is set while the job is still running, like a real end-stream request.
	release := make(chan struct{})
	stream := func(ctx context.Context, _ *slog.Logger, _ chan *protobuf.Notification, _ map[string]any, _ *metrics.Broker) error {
		rec.record("stream")
		<-release
		if ctx.Err() == nil {
			t.Error("job context was not cancelled by RequestStreamEnd")
		}
		return nil
	}

	job := r.RunAction(
		[]actions.Action{stream, rec.action("streamEnd")},
		[]actions.Action{rec.action("mkVod"), rec.action("checkVod"), rec.action("mkThumb")},
		map[string]any{}, r.log,
	)
	endJob(t, r, job, true)
	close(release)
	waitForJob(t, r, job)

	// StreamEnd still has to run, gocast learns the stream is over from it.
	assertCalls(t, rec.get(), []string{"stream", "streamEnd"})
}

func TestRunActionRunsVoDActionsWhenEndedWithoutDiscard(t *testing.T) {
	r := newTestRunner()
	rec := &recorder{}

	release := make(chan struct{})
	stream := func(_ context.Context, _ *slog.Logger, _ chan *protobuf.Notification, _ map[string]any, _ *metrics.Broker) error {
		rec.record("stream")
		<-release
		return nil
	}

	job := r.RunAction(
		[]actions.Action{stream, rec.action("streamEnd")},
		[]actions.Action{rec.action("mkVod")},
		map[string]any{}, r.log,
	)
	endJob(t, r, job, false)
	close(release)
	waitForJob(t, r, job)

	assertCalls(t, rec.get(), []string{"stream", "streamEnd", "mkVod"})
}

// An AbortingError stops the retries of the action that returned it, the following actions
// still run. Ending a stream early depends on that: Stream fails with the cancelled context
// and StreamEnd has to report the end anyway.
func TestRunActionAbortingErrorOnlyStopsRetriesOfThatAction(t *testing.T) {
	r := newTestRunner()
	rec := &recorder{}

	job := r.RunAction(
		[]actions.Action{
			rec.action("stream", actions.AbortingError(fmt.Errorf("killed"))),
			rec.action("streamEnd"),
		},
		[]actions.Action{rec.action("mkVod")},
		map[string]any{}, r.log,
	)
	waitForJob(t, r, job)

	assertCalls(t, rec.get(), []string{"stream", "streamEnd", "mkVod"})
}

func TestRunActionRetriesRecoverableErrors(t *testing.T) {
	r := newTestRunner()
	rec := &recorder{}

	job := r.RunAction(
		[]actions.Action{rec.action("stream", fmt.Errorf("flaky"), fmt.Errorf("flaky"))},
		nil, map[string]any{}, r.log,
	)
	waitForJob(t, r, job)

	assertCalls(t, rec.get(), []string{"stream", "stream", "stream"})
}

func TestRunActionCleansUpJobState(t *testing.T) {
	r := newTestRunner()
	rec := &recorder{}

	release := make(chan struct{})
	stream := func(_ context.Context, _ *slog.Logger, _ chan *protobuf.Notification, _ map[string]any, _ *metrics.Broker) error {
		<-release
		return nil
	}

	job := r.RunAction([]actions.Action{stream}, []actions.Action{rec.action("mkVod")}, map[string]any{}, r.log)
	endJob(t, r, job, true)

	r.jobsMu.Lock()
	_, tracked := r.jobs[job]
	discarded := r.discard[job]
	r.jobsMu.Unlock()
	if !tracked {
		t.Error("running job is not tracked in r.jobs")
	}
	if !discarded {
		t.Error("discard flag was not set by RequestStreamEnd")
	}

	close(release)
	waitForJob(t, r, job)

	r.jobsMu.Lock()
	defer r.jobsMu.Unlock()
	if len(r.jobs) != 0 {
		t.Errorf("jobs not cleaned up: %v", r.jobs)
	}
	if len(r.discard) != 0 {
		t.Errorf("discard flags not cleaned up: %v", r.discard)
	}
}

// The manager treats NotFound as "already ended", so the code matters, not just the error.
func TestRequestStreamEndUnknownJobIsNotFound(t *testing.T) {
	r := newTestRunner()

	resp, err := r.RequestStreamEnd(context.Background(), &protobuf.StreamEndRequest{
		JobId: ptr.Take("does-not-exist"),
	})
	if resp != nil {
		t.Errorf("got a response for an unknown job: %v", resp)
	}
	if got := status.Code(err); got != codes.NotFound {
		t.Errorf("status code: got %v, want %v", got, codes.NotFound)
	}
}

// Ending the same job repeatedly, as a double click on "end stream" does, must not race
// with the job goroutine cleaning itself up.
func TestRequestStreamEndIsSafeConcurrently(t *testing.T) {
	r := newTestRunner()

	release := make(chan struct{})
	stream := func(_ context.Context, _ *slog.Logger, _ chan *protobuf.Notification, _ map[string]any, _ *metrics.Broker) error {
		<-release
		return nil
	}
	job := r.RunAction([]actions.Action{stream}, nil, map[string]any{}, r.log)

	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, _ = r.RequestStreamEnd(context.Background(), &protobuf.StreamEndRequest{
				JobId:      ptr.Take(job),
				DiscardVod: ptr.Take(i%2 == 0),
			})
		}(i)
	}
	wg.Wait()
	close(release)
	waitForJob(t, r, job)
}

// drainBackoff counts how many retries b allows, giving up after limit.
func drainBackoff(b interface{ Next() (time.Duration, bool) }, limit int) int {
	for i := 0; i < limit; i++ {
		if _, stop := b.Next(); stop {
			return i
		}
	}
	return limit
}

func TestNotificationBackoffIsNotSharedBetweenNotifications(t *testing.T) {
	n := &protobuf.Notification{Data: &protobuf.Notification_Heartbeat{}}

	for i := 0; i < 3; i++ {
		if got := drainBackoff(notificationBackoff(n), 100); got != 10 {
			t.Fatalf("notification %d: got %d retries, want 10", i, got)
		}
	}
}

func TestNotificationBackoffCriticalRetriesIndefinitelyWithCap(t *testing.T) {
	n := &protobuf.Notification{Data: &protobuf.Notification_StreamEnd{}}

	// Exhaust one backoff; a fresh one for the next notification must start small again.
	drainBackoff(notificationBackoff(n), 100)
	b := notificationBackoff(n)
	if d, _ := b.Next(); d > 2*time.Second {
		t.Fatalf("first delay of fresh backoff is %v, want <= 2s", d)
	}
	for i := 0; i < 100; i++ {
		d, stop := b.Next()
		if stop {
			t.Fatalf("critical notification backoff stopped after %d retries", i)
		}
		if d > 30*time.Second {
			t.Fatalf("delay %v exceeds 30s cap", d)
		}
	}
}
