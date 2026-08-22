package apiv2

import (
	"net/http"

	grpcprom "github.com/grpc-ecosystem/go-grpc-middleware/providers/prometheus"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var serverMetrics = grpcprom.NewServerMetrics(
	grpcprom.WithServerHandlingTimeHistogram(
		grpcprom.WithHistogramBuckets([]float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10}),
	),
)

func init() {
	prometheus.MustRegister(serverMetrics)
}

// MetricsHandler serves the Prometheus exposition format for everything registered
// on the default registry, the gRPC server metrics included.
func MetricsHandler() http.Handler {
	return promhttp.Handler()
}
