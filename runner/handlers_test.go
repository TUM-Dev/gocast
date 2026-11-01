package runner

import "testing"

func TestRegenerateActions(t *testing.T) {
	if got := len(regenerateActions(false)); got != 2 {
		t.Errorf("without silence detection: %d actions, want 2", got)
	}
	if got := len(regenerateActions(true)); got != 3 {
		t.Errorf("with silence detection: %d actions, want 3", got)
	}
}
