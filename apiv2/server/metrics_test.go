package apiv2

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"google.golang.org/grpc"
)

func TestServerMetricsRecordsRPCs(t *testing.T) {
	interceptor := serverMetrics.UnaryServerInterceptor()
	info := &grpc.UnaryServerInfo{FullMethod: "/protobuf.MetaService/healthCheck"}
	handler := func(ctx context.Context, req any) (any, error) { return nil, nil }

	if _, err := interceptor(context.Background(), nil, info, handler); err != nil {
		t.Fatalf("interceptor returned an error: %v", err)
	}

	rec := httptest.NewRecorder()
	MetricsHandler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("scrape returned %d, want %d", rec.Code, http.StatusOK)
	}

	body := rec.Body.String()
	want := `grpc_server_handled_total{grpc_code="OK",grpc_method="healthCheck",grpc_service="protobuf.MetaService",grpc_type="unary"} 1`
	if !strings.Contains(body, want) {
		t.Errorf("scrape does not contain %q", want)
	}
	if !strings.Contains(body, "grpc_server_handling_seconds_bucket") {
		t.Error("scrape does not contain the handling time histogram")
	}
}
