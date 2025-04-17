package runner_manager

import (
	"testing"
)

func TestManagerOptions(t *testing.T) {
	m := Manager{}
	m.applyOpts([]Option{WithListenAddr(":1")})
	if m.listenAddr != ":1" {
		t.Errorf("m.listenAddr want: %v have: %v", ":1", m.listenAddr)
	}
}
