package tracing

import (
	"context"
	"slices"
	"testing"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

// otelgrpc injects and extracts through the global propagator; left at its no-op default,
// the manager's traces would end at the first call to a runner.
func TestSetupInstallsThePropagatorOnlyWhenEnabled(t *testing.T) {
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator())
	t.Cleanup(func() { otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator()) })

	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")
	if _, err := Setup(context.Background(), "test", "dev"); err != nil {
		t.Fatal(err)
	}
	if f := otel.GetTextMapPropagator().Fields(); len(f) != 0 {
		t.Fatalf("propagator %v installed without an endpoint", f)
	}

	// the exporter connects lazily, so no collector is needed
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://localhost:4317")
	shutdown, err := Setup(context.Background(), "test", "dev")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = shutdown(context.Background()) })
	if f := otel.GetTextMapPropagator().Fields(); !slices.Contains(f, "traceparent") {
		t.Errorf("propagator fields = %v, want traceparent", f)
	}
}
