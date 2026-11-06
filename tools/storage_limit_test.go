package tools

import (
	"errors"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestWithStorageSlotRejectsWhenFull(t *testing.T) {
	release := make(chan struct{})
	started := make(chan struct{}, maxStorageCalls)
	done := make(chan struct{}, maxStorageCalls)
	for range maxStorageCalls {
		go func() {
			_, _ = withStorageSlot(func() (int, error) {
				started <- struct{}{}
				<-release
				return 0, nil
			})
			done <- struct{}{}
		}()
	}
	for range maxStorageCalls {
		<-started
	}

	before := testutil.ToFloat64(storageCallsRejected)
	called := false
	_, err := withStorageSlot(func() (int, error) { called = true; return 0, nil })
	if !errors.Is(err, ErrStorageBusy) || called {
		t.Fatalf("want ErrStorageBusy without running fn, got err=%v called=%v", err, called)
	}
	if got := testutil.ToFloat64(storageCallsRejected) - before; got != 1 {
		t.Fatalf("rejected counter grew by %v, want 1", got)
	}
	if got := testutil.ToFloat64(storageCallsInFlight); got != maxStorageCalls {
		t.Fatalf("in flight = %v, want %d", got, maxStorageCalls)
	}

	close(release)
	for range maxStorageCalls {
		<-done
	}
	if _, err := withStorageSlot(func() (int, error) { return 1, nil }); err != nil {
		t.Fatalf("slots not released: %v", err)
	}
}
