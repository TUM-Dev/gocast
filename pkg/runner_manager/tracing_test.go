package runner_manager

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
	"go.uber.org/mock/gomock"
	"gorm.io/gorm"

	"github.com/tum-dev/gocast/runner/pkg/ptr"
	"github.com/tum-dev/gocast/runner/protobuf"

	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/mock_dao"
	"github.com/TUM-Dev/gocast/model"
)

func recordSpans(t *testing.T) *tracetest.SpanRecorder {
	t.Helper()
	rec := tracetest.NewSpanRecorder()
	otel.SetTracerProvider(sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(rec)))
	t.Cleanup(func() { otel.SetTracerProvider(noop.NewTracerProvider()) })
	return rec
}

func traceIDOf(ctx context.Context) trace.TraceID {
	return trace.SpanContextFromContext(ctx).TraceID()
}

// The steps of a stream version find each other's trace without being told: that is the
// only link between a cron tick, a job on a runner and an admin ending it hours later.
func TestWithStreamTraceIsStablePerStreamVersion(t *testing.T) {
	comb := protobuf.StreamVersion_STREAM_VERSION_COMBINED
	cam := protobuf.StreamVersion_STREAM_VERSION_CAMERA

	a := traceIDOf(withStreamTrace(context.Background(), 7, comb))
	if !a.IsValid() {
		t.Fatal("trace id is not valid")
	}
	if b := traceIDOf(withStreamTrace(context.Background(), 7, comb)); a != b {
		t.Errorf("same stream version: %s != %s", a, b)
	}
	if b := traceIDOf(withStreamTrace(context.Background(), 7, cam)); a == b {
		t.Error("two versions of a stream share a trace")
	}
	if b := traceIDOf(withStreamTrace(context.Background(), 8, comb)); a == b {
		t.Error("two streams share a trace")
	}
}

// What a runner reports joins the trace of its job; an older runner that sends no
// traceparent lands in the one of the stream version it reports on.
func TestNotifyJoinsTheTrace(t *testing.T) {
	vodReady := func(traceparent string) *protobuf.Notification {
		n := &protobuf.Notification{Data: &protobuf.Notification_VodReady{VodReady: &protobuf.VODReadyNotification{
			Stream:        &protobuf.StreamInfo{Id: ptr.Take(uint64(5))},
			StreamVersion: ptr.Take(protobuf.StreamVersion_STREAM_VERSION_COMBINED),
		}}}
		if traceparent != "" {
			n.TraceParent = ptr.Take(traceparent)
		}
		return n
	}
	notify := func(t *testing.T, n *protobuf.Notification) *tracetest.SpanRecorder {
		t.Helper()
		spans := recordSpans(t)
		ctrl := gomock.NewController(t)
		streamsDao := mock_dao.NewMockStreamsDao(ctrl)
		streamsDao.EXPECT().GetStreamByID(gomock.Any(), "5").Return(model.Stream{}, gorm.ErrRecordNotFound).AnyTimes()
		m := New(dao.DaoWrapper{StreamsDao: streamsDao})
		_, _ = m.Notify(context.Background(), n)
		return spans
	}

	t.Run("with a traceparent", func(t *testing.T) {
		const tp = "00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01"
		ended := notify(t, vodReady(tp)).Ended()
		if len(ended) != 1 || ended[0].Name() != "manager.notify VodReady" {
			t.Fatalf("spans = %v, want manager.notify VodReady", ended)
		}
		if got := ended[0].SpanContext().TraceID().String(); got != "0af7651916cd43dd8448eb211c80319c" {
			t.Errorf("trace = %s, want the runner's", got)
		}
		if got := ended[0].Parent().SpanID().String(); got != "b7ad6b7169203331" {
			t.Errorf("parent = %s, want the runner's span", got)
		}
	})

	t.Run("without one", func(t *testing.T) {
		ended := notify(t, vodReady("")).Ended()
		if len(ended) != 1 {
			t.Fatalf("%d spans, want 1", len(ended))
		}
		want := traceIDOf(withStreamTrace(context.Background(), 5, protobuf.StreamVersion_STREAM_VERSION_COMBINED))
		if got := ended[0].SpanContext().TraceID(); got != want {
			t.Errorf("trace = %s, want the stream version's %s", got, want)
		}
	})

	t.Run("with a malformed one", func(t *testing.T) {
		ended := notify(t, vodReady("garbage")).Ended()
		if len(ended) != 1 {
			t.Fatalf("%d spans, want 1", len(ended))
		}
		want := traceIDOf(withStreamTrace(context.Background(), 5, protobuf.StreamVersion_STREAM_VERSION_COMBINED))
		if got := ended[0].SpanContext().TraceID(); got != want {
			t.Errorf("trace = %s, want the stream version's %s", got, want)
		}
	})

	t.Run("a heartbeat is not traced", func(t *testing.T) {
		spans := recordSpans(t)
		ctrl := gomock.NewController(t)
		runnerDao := mock_dao.NewMockRunnerDao(ctrl)
		runnerDao.EXPECT().Get(gomock.Any(), "r1").Return(model.Runner{}, gorm.ErrRecordNotFound)
		m := New(dao.DaoWrapper{RunnerDao: runnerDao})
		_, _ = m.Notify(context.Background(), &protobuf.Notification{Data: &protobuf.Notification_Heartbeat{
			Heartbeat: &protobuf.HeartbeatNotification{Hostname: ptr.Take("r1")},
		}})
		if n := len(spans.Ended()); n != 0 {
			t.Errorf("%d spans for a heartbeat, want 0", n)
		}
	})
}
