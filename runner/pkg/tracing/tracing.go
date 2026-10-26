// Package tracing exports OpenTelemetry traces of the runner's jobs.
//
// It is off unless an OTLP endpoint is configured through the standard OTEL_EXPORTER_OTLP_*
// environment variables, so a runner without one (development, tests) pays nothing: the
// global tracer provider stays the no-op default.
package tracing

import (
	"context"
	"fmt"
	"os"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

const tracerName = "github.com/tum-dev/gocast/runner"

// propagator is the W3C trace context, the format the manager and the runner exchange in
// Notification.trace_parent.
var propagator = propagation.TraceContext{}

// Enabled tells whether the environment asks for traces to be exported.
func Enabled() bool {
	return os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT") != "" || os.Getenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT") != ""
}

// Setup installs the global tracer provider and returns the func that flushes and stops it.
// Without an endpoint it does nothing and the returned func is a no-op.
func Setup(ctx context.Context, service, version string) (shutdown func(context.Context) error, err error) {
	if !Enabled() {
		return func(context.Context) error { return nil }, nil
	}
	exp, err := otlptracegrpc.New(ctx)
	if err != nil {
		return nil, fmt.Errorf("create otlp exporter: %w", err)
	}
	res, err := resource.New(ctx,
		resource.WithAttributes(
			attribute.String("service.name", service),
			attribute.String("service.version", version),
		),
		resource.WithHost(),
		// last, so that OTEL_SERVICE_NAME and OTEL_RESOURCE_ATTRIBUTES win
		resource.WithFromEnv(),
	)
	if err != nil {
		return nil, fmt.Errorf("create otel resource: %w", err)
	}
	// Every span is kept, there are only a handful of jobs per lecture.
	tp := sdktrace.NewTracerProvider(sdktrace.WithBatcher(exp), sdktrace.WithResource(res))
	otel.SetTracerProvider(tp)
	// The otelgrpc handlers inject and extract through the global propagator, which is a
	// no-op until set: without it no trace crosses the manager's calls to the runner.
	otel.SetTextMapPropagator(propagator)
	return tp.Shutdown, nil
}

// Tracer is the runner's tracer.
func Tracer() trace.Tracer { return otel.Tracer(tracerName) }

// Traceparent returns the W3C traceparent of the span in ctx, or "" if there is none.
func Traceparent(ctx context.Context) string {
	c := propagation.MapCarrier{}
	propagator.Inject(ctx, c)
	return c.Get("traceparent")
}

// WithTraceparent returns ctx with the remote span context of traceparent as its parent.
// A malformed or empty traceparent leaves ctx as it was.
func WithTraceparent(ctx context.Context, traceparent string) context.Context {
	if traceparent == "" {
		return ctx
	}
	return propagator.Extract(ctx, propagation.MapCarrier{"traceparent": traceparent})
}

// Detach returns a context that carries only the span context of parent. The jobs of the
// runner outlive the request that started them, so they must not inherit its cancellation,
// but their spans still belong to the trace of the request.
func Detach(parent context.Context) context.Context {
	return trace.ContextWithRemoteSpanContext(context.Background(), trace.SpanContextFromContext(parent))
}
