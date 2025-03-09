package actions

import (
	"fmt"
	"testing"
)

func TestAbortingError(t *testing.T) {
	err := AbortingError(fmt.Errorf("some error"))
	if err == nil {
		t.Errorf("error should not be nil")
	}
	if err.Error() != "aborting: some error" {
		t.Errorf("error should be 'aborting: some error'")
	}
	if !IsAbortingError(err) {
		t.Errorf("error should be retryable")
	}
}
