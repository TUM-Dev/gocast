package runner

import "testing"

func TestRegenerateActions(t *testing.T) {
	if got := len(regenerateActions(false)); got != 1 {
		t.Errorf("without silence detection: %d actions, want 1", got)
	}
	if got := len(regenerateActions(true)); got != 2 {
		t.Errorf("with silence detection: %d actions, want 2", got)
	}
}
