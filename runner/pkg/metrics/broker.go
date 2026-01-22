// Package metrics provides Prometheus compatible monitoring.
package metrics

import (
	"fmt"
	"log/slog"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Broker manages Prometheus metrics.
type Broker struct {
	port                 int
	Streams              *prometheus.GaugeVec
	StreamErrors         *prometheus.CounterVec
	ConvertingProgresses *prometheus.GaugeVec
	ConvertingErrors     *prometheus.CounterVec
}

// Option represents a functional option for configuring a Broker.
type Option func(broker *Broker)

// NewBroker initializes a new Broker with optional configurations.
// It sets up Prometheus all available metrics.
//
// Example usage:
//
//	b := metrics.NewBroker(metrics.WithPort(8080))
//	go b.Run()
//
//	b.Streams.With(b.With().Stream(123).Input("rtmp://1.2.3.4/src")).Set(5)  // Set active streams
func NewBroker(options ...Option) *Broker {
	b := &Broker{
		port: 9947,
		Streams: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: "runner",
			Subsystem: "stream",
			Name:      "n_streams",
			Help:      "Number of active streams",
		}, []string{"stream_id", "source"}),

		StreamErrors: promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: "runner",
			Subsystem: "stream",
			Name:      "n_errors",
			Help:      "Number of stream ffmpeg errors",
		}, []string{"stream_id", "source"}),

		ConvertingProgresses: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: "runner",
			Subsystem: "converting",
			Name:      "n_converting",
			Help:      "Number of streams currently being converted",
		}, []string{"stream_id"}),
		ConvertingErrors: promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: "runner",
			Subsystem: "converting",
			Name:      "n_converting_errs",
			Help:      "Number of failures during conversion",
		}, []string{"stream_id", "stream_version"}),
	}
	for _, option := range options {
		option(b)
	}
	return b
}

// WithPort configures the Broker to listen on a custom port.
func WithPort(port int) Option {
	return func(broker *Broker) {
		broker.port = port
	}
}

// Run starts an HTTP server that exposes the metrics.
func (b *Broker) Run() {
	http.Handle("/metrics", promhttp.Handler())
	slog.Error("Serving metrics", "err", http.ListenAndServe(fmt.Sprintf(":%d", b.port), nil))
}

// LabelBuilder helps construct Prometheus labels dynamically.
type LabelBuilder prometheus.Labels

// L converts LabelBuilder to a Prometheus Labels map.
func (b LabelBuilder) L() prometheus.Labels {
	return prometheus.Labels(b)
}

// With initializes a new LabelBuilder.
func (b *Broker) With() LabelBuilder {
	return LabelBuilder{}
}

// Stream adds a "stream_id" label to LabelBuilder.
func (b LabelBuilder) Stream(streamID uint64) LabelBuilder {
	b["stream_id"] = fmt.Sprintf("%d", streamID)
	return b
}

// Source adds a "source" label to LabelBuilder.
func (b LabelBuilder) Source(source string) LabelBuilder {
	b["source"] = source
	return b
}

// StreamVersion adds a "stream_version" label to LabelBuilder.
func (b LabelBuilder) StreamVersion(version string) LabelBuilder {
	b["stream_version"] = version
	return b
}
