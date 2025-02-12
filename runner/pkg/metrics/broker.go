package metrics

import (
	"fmt"
	"log/slog"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Broker struct {
	port int

	Streams      *prometheus.GaugeVec
	StreamErrors *prometheus.CounterVec
}

type Option func(broker *Broker)

func NewBroker(options ...Option) *Broker {
	b := &Broker{
		port: 9947,
		Streams: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: "runner",
			Subsystem: "stream",
			Name:      "n_streams",
			Help:      "Number of streams active",
		}, []string{"stream_id", "input"}),

		StreamErrors: promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: "runner",
			Subsystem: "stream",
			Name:      "n_errors",
			Help:      "Number of stream ffmpegs erroring",
		}, []string{"stream_id", "input"}),
	}
	for _, option := range options {
		option(b)
	}
	return b
}

func WithPort(port int) Option {
	return func(broker *Broker) {
		broker.port = port
	}
}

func (b *Broker) Run() {
	http.Handle("/metrics", promhttp.Handler())
	slog.Error("Serving metrics", "err", http.ListenAndServe(fmt.Sprintf(":%d", b.port), promhttp.Handler()))
}

type LabelBuilder prometheus.Labels

func (b LabelBuilder) L() prometheus.Labels {
	return prometheus.Labels(b)
}

func (b *Broker) With() LabelBuilder {
	return LabelBuilder{}
}

func (b LabelBuilder) Stream(streamID uint64) LabelBuilder {
	b["stream_id"] = fmt.Sprintf("%d", streamID)
	return b
}

func (b LabelBuilder) Source(source string) LabelBuilder {
	b["source"] = source
	return b
}
