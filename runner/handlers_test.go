package runner

import (
	"reflect"
	"testing"

	"github.com/tum-dev/gocast/runner/pkg/actions"
)

func TestRegenerateActions(t *testing.T) {
	if got := len(regenerateActions(false)); got != 2 {
		t.Errorf("without silence detection: %d actions, want 2", got)
	}
	if got := len(regenerateActions(true)); got != 3 {
		t.Errorf("with silence detection: %d actions, want 3", got)
	}
}

// A thumbnail that can't be made aborts the job, and the recordings that are redone are the
// ones that may well have none: the duration must not depend on it.
func TestRegenerateActionsProbeDurationBeforeThumbnail(t *testing.T) {
	for _, silence := range []bool{false, true} {
		got := regenerateActions(silence)
		if reflect.ValueOf(got[0]).Pointer() != reflect.ValueOf(actions.MkDuration).Pointer() {
			t.Errorf("detectSilence=%v: the first action is not MkDuration", silence)
		}
	}
}
