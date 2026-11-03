package runner

import (
	"reflect"
	"testing"

	"github.com/tum-dev/gocast/runner/pkg/actions"
)

func TestRegenerateActions(t *testing.T) {
	for _, tc := range []struct {
		silence, sprite bool
		want            int
	}{
		{false, false, 2},
		{true, false, 3},
		{false, true, 3},
		{true, true, 4},
	} {
		if got := len(regenerateActions(tc.silence, tc.sprite)); got != tc.want {
			t.Errorf("silence=%v sprite=%v: %d actions, want %d", tc.silence, tc.sprite, got, tc.want)
		}
	}
}

// A thumbnail that can't be made aborts the job, and the recordings that are redone are the
// ones that may well have none: the duration must not depend on it.
func TestRegenerateActionsProbeDurationBeforeThumbnail(t *testing.T) {
	for _, silence := range []bool{false, true} {
		got := regenerateActions(silence, true)
		if reflect.ValueOf(got[0]).Pointer() != reflect.ValueOf(actions.MkDuration).Pointer() {
			t.Errorf("detectSilence=%v: the first action is not MkDuration", silence)
		}
	}
}
