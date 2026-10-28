package runner

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"

	"github.com/tum-dev/gocast/runner/pkg/actions"
	"github.com/tum-dev/gocast/runner/pkg/metrics"
	"github.com/tum-dev/gocast/runner/pkg/tracing"
	"github.com/tum-dev/gocast/runner/protobuf"
)

// recordSpans swaps the global tracer provider for one that keeps its spans in memory.
func recordSpans(t *testing.T) *tracetest.SpanRecorder {
	t.Helper()
	rec := tracetest.NewSpanRecorder()
	otel.SetTracerProvider(sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(rec)))
	// what the runner has without an endpoint, other tests must not see the recorder
	t.Cleanup(func() { otel.SetTracerProvider(noop.NewTracerProvider()) })
	return rec
}

func spansNamed(rec *tracetest.SpanRecorder, name string) []sdktrace.ReadOnlySpan {
	var out []sdktrace.ReadOnlySpan
	for _, s := range rec.Ended() {
		if s.Name() == name {
			out = append(out, s)
		}
	}
	return out
}

// A job joins the trace of the request that started it, with a span per phase and one per
// attempt of every action, and stamps what it notifies with that trace.
func TestRunActionContextTracesTheJob(t *testing.T) {
	spans := recordSpans(t)
	r := newTestRunner()
	rec := &recorder{}

	// the manager's request, whose cancellation must not reach the job
	reqCtx, req := tracing.Tracer().Start(context.Background(), "manager.request")
	reqCtx, cancelReq := context.WithCancel(reqCtx)
	req.End()
	cancelReq()

	notifying := func(ctx context.Context, _ *slog.Logger, notify chan *protobuf.Notification, _ map[string]any, _ *metrics.Broker) error {
		rec.record("vod")
		notify <- &protobuf.Notification{Data: &protobuf.Notification_VodReady{VodReady: &protobuf.VODReadyNotification{}}}
		return nil
	}
	job := r.RunActionContext(reqCtx,
		[]actions.Action{rec.action("stream", errors.New("camera unreachable"))},
		[]actions.Action{notifying},
		map[string]any{"streamID": uint64(7), "streamVersion": "STREAM_VERSION_COMB"}, r.log)
	waitForJob(t, r, job)

	traceID := req.SpanContext().TraceID()
	for _, name := range []string{"runner.job", "runner.stream", "runner.vod", "action.retry_wait"} {
		got := spansNamed(spans, name)
		if len(got) != 1 {
			t.Fatalf("%d %q spans, want 1", len(got), name)
		}
		if got[0].SpanContext().TraceID() != traceID {
			t.Errorf("%q is in trace %s, want the request's %s", name, got[0].SpanContext().TraceID(), traceID)
		}
	}
	var attempts int
	for _, s := range spans.Ended() {
		if strings.HasPrefix(s.Name(), "action.") && s.Name() != "action.retry_wait" {
			attempts++
		}
	}
	if attempts != 3 { // the failed and the successful attempt of stream, and the vod action
		t.Errorf("%d action spans, want 3", attempts)
	}

	n := <-r.notifications
	parent := trace.SpanContextFromContext(tracing.WithTraceparent(context.Background(), n.GetTraceParent()))
	if parent.TraceID() != traceID {
		t.Errorf("notification carries trace %q, want one in %s", n.GetTraceParent(), traceID)
	}
	if parent.SpanID() != spansNamed(spans, "runner.job")[0].SpanContext().SpanID() {
		t.Errorf("notification is not parented on the job span: %q", n.GetTraceParent())
	}
}

// Without an endpoint nothing is exported and notifications go out unchanged.
func TestRunActionWithoutTracingStampsNothing(t *testing.T) {
	r := newTestRunner()
	notifying := func(_ context.Context, _ *slog.Logger, notify chan *protobuf.Notification, _ map[string]any, _ *metrics.Broker) error {
		notify <- &protobuf.Notification{Data: &protobuf.Notification_StreamEnd{StreamEnd: &protobuf.StreamEndNotification{}}}
		return nil
	}
	waitForJob(t, r, r.RunAction([]actions.Action{notifying}, nil, map[string]any{}, r.log))

	if tp := (<-r.notifications).GetTraceParent(); tp != "" {
		t.Errorf("notification carries trace %q without tracing", tp)
	}
}
