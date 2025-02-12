package metrics

import "testing"

func TestLabelBuilder(t *testing.T) {
	b := NewBroker()
	if b == nil {
		t.Errorf("broker unexpectedly nil")
		return
	}
	if b.StreamErrors == nil {
		t.Errorf("broker metric not initialized")
	}
	labels := b.With().Stream(123).Source("test").L()
	if labels["stream_id"] != "123" || labels["source"] != "test" {
		t.Errorf("Unexpected labels: %+v", labels)
	}
}
