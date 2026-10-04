package metrics

import (
	"strings"
	"testing"
)

// promauto registers every metric on the default registry, so NewBroker may
// only be called once per test binary; both checks share one broker.
func TestBroker(t *testing.T) {
	b := NewBroker(WithHostname("vm123"), WithVersion("v0.1.2"))
	if b == nil {
		t.Fatalf("broker unexpectedly nil")
	}

	t.Run("LabelBuilder", func(t *testing.T) {
		if b.StreamErrors == nil {
			t.Errorf("broker metric not initialized")
		}
		labels := b.With().Stream(123).Source("test").L()
		if labels["stream_id"] != "123" || labels["source"] != "test" {
			t.Errorf("Unexpected labels: %+v", labels)
		}
	})

	t.Run("UptimeLabels", func(t *testing.T) {
		desc := b.Uptime.Desc().String()
		if !strings.Contains(desc, `host="vm123"`) || !strings.Contains(desc, `version="v0.1.2"`) {
			t.Errorf("uptime metric missing host/version labels: %s", desc)
		}
	})
}
