package runner_manager

import (
	"context"
	"crypto/sha256"
	"fmt"
	"strings"

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc/filters"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"

	"github.com/tum-dev/gocast/runner/pkg/tracing"
	"github.com/tum-dev/gocast/runner/protobuf"
)

// tracer is looked up on use: a tracer taken from the global provider before an SDK
// provider was installed keeps delegating to the first one.
func tracer() trace.Tracer { return otel.Tracer("github.com/TUM-Dev/gocast/pkg/runner_manager") }

// runnerClientTracing continues the manager's traces in the runner. The live preview is
// left out: it is requested every minute for every running stream and belongs to no trace.
var runnerClientTracing = grpc.WithStatsHandler(otelgrpc.NewClientHandler(otelgrpc.WithFilter(
	filters.Not(filters.MethodName("GenerateLivePreview")),
)))

// streamTraceID is the trace of one version of a stream, which is what ties the whole flow
// together. Its steps are hours apart and happen in different processes (the cron tick that
// starts it, the runner's job, the notifications that report its results, an admin ending
// it), and none of them hands its context to the next. The trace id is therefore derived
// from the stream and its version, so every step finds the same trace without anything
// being stored, across restarts of the manager and the runner alike.
//
// A version of 0 (STREAM_VERSION_UNSPECIFIED) is the stream as a whole, for the jobs that
// belong to no version such as section images.
func streamTraceID(streamID uint64, version protobuf.StreamVersion) trace.TraceID {
	sum := sha256.Sum256([]byte(fmt.Sprintf("gocast/stream/%d/%d", streamID, version)))
	return trace.TraceID(sum[:16])
}

// withStreamTrace makes the span started next with ctx a root of the stream version's
// trace, see streamTraceID. The steps are separate roots of one trace: they have no common
// parent, and a made-up one would never be exported.
func withStreamTrace(ctx context.Context, streamID uint64, version protobuf.StreamVersion) context.Context {
	return tracing.WithTraceID(ctx, streamTraceID(streamID, version))
}

// startSpan starts a span named name below whatever trace ctx is in.
func startSpan(ctx context.Context, name string, attrs ...attribute.KeyValue) (context.Context, trace.Span) {
	return tracer().Start(ctx, name, trace.WithAttributes(attrs...))
}

// endSpan ends span, marking it failed if *err is set. Meant for defer with a named result.
func endSpan(span trace.Span, err *error) {
	if *err != nil {
		span.RecordError(*err)
		span.SetStatus(codes.Error, (*err).Error())
	}
	span.End()
}

func streamAttrs(streamID uint64, version protobuf.StreamVersion) []attribute.KeyValue {
	attrs := []attribute.KeyValue{attribute.Int64("stream.id", int64(streamID))}
	if version != protobuf.StreamVersion_STREAM_VERSION_UNSPECIFIED {
		attrs = append(attrs, attribute.String("stream.version", version.String()))
	}
	return attrs
}

// notificationStream is the stream version n reports on, for the notifications that carry one.
func notificationStream(n *protobuf.Notification) (id uint64, version protobuf.StreamVersion, ok bool) {
	switch d := n.Data.(type) {
	case *protobuf.Notification_StreamStart:
		return d.StreamStart.GetStream().GetId(), d.StreamStart.GetStreamVersion(), true
	case *protobuf.Notification_StreamEnd:
		return d.StreamEnd.GetStream().GetId(), d.StreamEnd.GetStreamVersion(), true
	case *protobuf.Notification_VodReady:
		return d.VodReady.GetStream().GetId(), d.VodReady.GetStreamVersion(), true
	case *protobuf.Notification_ThumbnailReady:
		return d.ThumbnailReady.GetStream().GetId(), d.ThumbnailReady.GetStreamVersion(), true
	case *protobuf.Notification_TranscodingProgress:
		return d.TranscodingProgress.GetStream().GetId(), d.TranscodingProgress.GetStreamVersion(), true
	case *protobuf.Notification_VodFailed:
		return d.VodFailed.GetStream().GetId(), d.VodFailed.GetStreamVersion(), true
	case *protobuf.Notification_SilenceDetected:
		return d.SilenceDetected.GetStream().GetId(), d.SilenceDetected.GetStreamVersion(), true
	case *protobuf.Notification_SectionImagesReady:
		return d.SectionImagesReady.GetStream().GetId(), protobuf.StreamVersion_STREAM_VERSION_UNSPECIFIED, true
	}
	return 0, 0, false
}

// startNotifySpan starts the span for handling n. The runner names the span it delivered n
// in; a runner that doesn't (an older one) still lands in the right trace through the
// stream version n reports on.
func startNotifySpan(ctx context.Context, n *protobuf.Notification) (context.Context, trace.Span) {
	var attrs []attribute.KeyValue
	id, version, hasStream := notificationStream(n)
	if hasStream {
		attrs = streamAttrs(id, version)
	}
	// A traceparent that doesn't parse is as good as none: it must not cost the stream
	// version's trace.
	if remote := tracing.WithTraceparent(ctx, n.GetTraceParent()); trace.SpanContextFromContext(remote).IsValid() {
		ctx = remote
	} else if hasStream {
		ctx = withStreamTrace(ctx, id, version)
	}
	name := strings.TrimPrefix(fmt.Sprintf("%T", n.Data), "*protobuf.Notification_")
	return tracer().Start(ctx, "manager.notify "+name,
		trace.WithSpanKind(trace.SpanKindConsumer), trace.WithAttributes(attrs...))
}
