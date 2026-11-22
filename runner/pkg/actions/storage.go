package actions

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"sync"
	"syscall"
	"time"
)

// errStorageUnresponsive is what a call into the storage returns when it did not answer in
// time. The call itself may still be running, see storageGate.
var errStorageUnresponsive = errors.New("storage did not respond")

// errDraining is what waitForStorage returns when the runner drains while it waits.
var errDraining = errors.New("runner is draining")

// isOutage tells whether err from a call into the storage is the storage being
// unavailable, which passes, as opposed to an error a retry can't change, like a
// read-only or full file system or a wrong StoragePath.
func isOutage(err error) bool {
	for _, e := range []error{errStorageUnresponsive, syscall.EIO, syscall.ESTALE, syscall.ENOTCONN, syscall.ESHUTDOWN} {
		if errors.Is(err, e) {
			return true
		}
	}
	return false
}

// storageMkdirAll is os.MkdirAll for a directory in the storage, through the storageGate.
func storageMkdirAll(ctx context.Context, dir string) error {
	return storage.do(ctx, storageTimeout, func() error { return os.MkdirAll(dir, os.ModePerm) })
}

// The timeout of a single call into the storage and the bounds of the wait between two.
// Variables so the tests can swap them.
var (
	storageTimeout      = 30 * time.Second
	storageRetryInitial = 30 * time.Second
	storageRetryLongest = 5 * time.Minute
)

// storageGate runs at most one filesystem call into the storage at a time.
//
// The storage is a CephFS mount. While its MDS is down, a call into it blocks in the kernel
// until the MDS is back, and cancelling a context does not end it. Each such call
// pins an OS thread, and the Go runtime dies at 10000 of them. A caller that gave up on its
// call and tried again would add one per attempt, so a caller that finds a call still in
// flight waits for it instead of starting its own.
type storageGate struct {
	mu       sync.Mutex
	inFlight chan struct{} // closed once the call in flight returned, nil without one
}

var storage = &storageGate{}

// do runs fn and returns its error, or errStorageUnresponsive if fn, or the call in flight
// before it, did not return within timeout.
func (g *storageGate) do(ctx context.Context, timeout time.Duration, fn func() error) error {
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	g.mu.Lock()
	for g.inFlight != nil {
		busy := g.inFlight
		g.mu.Unlock()
		select {
		case <-busy:
		case <-deadline.C:
			return errStorageUnresponsive
		case <-ctx.Done():
			return ctx.Err()
		}
		g.mu.Lock()
	}
	done := make(chan struct{})
	g.inFlight = done
	g.mu.Unlock()

	var err error
	go func() {
		err = fn()
		g.mu.Lock()
		g.inFlight = nil
		g.mu.Unlock()
		close(done)
	}()
	select {
	case <-done:
		return err
	case <-deadline.C:
		return errStorageUnresponsive
	case <-ctx.Done():
		return ctx.Err()
	}
}

// waitForStorage creates dir in the storage, for as long as it takes the storage to come
// back: the recording stays on the local disk meanwhile, and a VoD that is late beats one
// that someone has to make by hand. The wait ends early with the end of ctx, with
// errDraining once drained is closed, and with any error that is not an outage.
//
// Giving up on drain is safe because the VoDPendingFile stays, so the next start of the
// runner makes the VoD. Waiting would hold up the shutdown for as long as the storage is
// down.
func waitForStorage(ctx context.Context, drained <-chan struct{}, log *slog.Logger, dir string) error {
	wait := storageRetryInitial
	for {
		err := storageMkdirAll(ctx, dir)
		if err == nil {
			return nil
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if !isOutage(err) {
			return err
		}
		log.Warn("storage unavailable, waiting for it", "dir", dir, "error", err, "retry_in", wait.String())
		t := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			t.Stop()
			return ctx.Err()
		case <-drained:
			t.Stop()
			return errDraining
		case <-t.C:
		}
		wait = min(wait*2, storageRetryLongest)
	}
}
