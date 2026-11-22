package actions

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
)

// A call that hangs, as one into a CephFS mount without its MDS does, must not be joined by
// another one per attempt: each pins an OS thread for as long as the storage is down.
func TestStorageGateStartsNoSecondCallWhileOneHangs(t *testing.T) {
	g := &storageGate{}
	release := make(chan struct{})
	var calls atomic.Int32
	hang := func() error {
		calls.Add(1)
		<-release
		return nil
	}

	for range 3 {
		if err := g.do(context.Background(), 10*time.Millisecond, hang); !errors.Is(err, errStorageUnresponsive) {
			t.Fatalf("err = %v, want errStorageUnresponsive", err)
		}
	}
	if got := calls.Load(); got != 1 {
		t.Errorf("%d calls started while the first hung, want 1", got)
	}

	close(release)
	if err := g.do(context.Background(), time.Second, func() error { calls.Add(1); return nil }); err != nil {
		t.Fatalf("after the storage is back: %v", err)
	}
	if got := calls.Load(); got != 2 {
		t.Errorf("%d calls, want 2", got)
	}
}

func TestStorageGateReturnsTheError(t *testing.T) {
	want := errors.New("input/output error")
	if err := (&storageGate{}).do(context.Background(), time.Second, func() error { return want }); !errors.Is(err, want) {
		t.Errorf("err = %v, want %v", err, want)
	}
}

// The storage being down only delays the VoD: waitForStorage keeps going until it is back.
func TestWaitForStorageWaitsUntilTheStorageIsBack(t *testing.T) {
	prevTimeout, prevMin, prevMax, prevGate := storageTimeout, storageRetryInitial, storageRetryLongest, storage
	storageTimeout, storageRetryInitial, storageRetryLongest = 10*time.Millisecond, time.Millisecond, 2*time.Millisecond
	t.Cleanup(func() {
		storageTimeout, storageRetryInitial, storageRetryLongest, storage = prevTimeout, prevMin, prevMax, prevGate
	})
	// The first call hangs past its timeout, as with a dead mount, and returns after a while.
	g := &storageGate{}
	release := make(chan struct{})
	g.inFlight = release
	storage = g
	go func() {
		time.Sleep(50 * time.Millisecond)
		g.mu.Lock()
		g.inFlight = nil
		g.mu.Unlock()
		close(release)
	}()

	dir := filepath.Join(t.TempDir(), "1", "STREAM_VERSION_COMBINED")
	if err := waitForStorage(context.Background(), nil, slog.New(slog.NewTextHandler(io.Discard, nil)), dir); err != nil {
		t.Fatal(err)
	}
}

func TestWaitForStorageEndsWithTheJob(t *testing.T) {
	prevGate := storage
	t.Cleanup(func() { storage = prevGate })
	storage = &storageGate{inFlight: make(chan struct{})} // never comes back

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := waitForStorage(ctx, nil, slog.New(slog.NewTextHandler(io.Discard, nil)), t.TempDir())
	if !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want context.Canceled", err)
	}
}

// A storage that answers with an error a retry can't change, like a read-only file
// system, ends the wait at once, as before there was one.
func TestWaitForStorageGivesUpOnPermanentErrors(t *testing.T) {
	file := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(file, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		done <- waitForStorage(context.Background(), nil, slog.New(slog.NewTextHandler(io.Discard, nil)), filepath.Join(file, "vod"))
	}()
	select {
	case err := <-done:
		if err == nil || isOutage(err) {
			t.Errorf("err = %v, want a permanent error", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("waitForStorage retried a permanent error")
	}
}

func TestIsOutage(t *testing.T) {
	for err, want := range map[error]bool{
		errStorageUnresponsive: true,
		&os.PathError{Op: "mkdir", Path: "/mass/1", Err: syscall.EIO}:      true,
		&os.PathError{Op: "mkdir", Path: "/mass/1", Err: syscall.ENOTCONN}: true,
		&os.PathError{Op: "mkdir", Path: "/mass/1", Err: syscall.EACCES}:   false,
		&os.PathError{Op: "mkdir", Path: "/mass/1", Err: syscall.EROFS}:    false,
		&os.PathError{Op: "mkdir", Path: "/mass/1", Err: syscall.ENOSPC}:   false,
	} {
		if got := isOutage(err); got != want {
			t.Errorf("isOutage(%v) = %v, want %v", err, got, want)
		}
	}
}

// A drain must not wait for the storage: the next start makes the VoD.
func TestWaitForStorageEndsWithTheDrain(t *testing.T) {
	prevTimeout, prevInitial, prevGate := storageTimeout, storageRetryInitial, storage
	storageTimeout, storageRetryInitial = 10*time.Millisecond, time.Hour
	t.Cleanup(func() { storageTimeout, storageRetryInitial, storage = prevTimeout, prevInitial, prevGate })
	storage = &storageGate{inFlight: make(chan struct{})} // never comes back

	drained := make(chan struct{})
	close(drained)
	err := waitForStorage(context.Background(), drained, slog.New(slog.NewTextHandler(io.Discard, nil)), t.TempDir())
	if !errors.Is(err, errDraining) {
		t.Errorf("err = %v, want errDraining", err)
	}
}
