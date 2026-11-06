// Package storagelimit caps the calls that touch the mass storage.
package storagelimit

import (
	"errors"
	"os"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// maxStorageCalls caps the file-system calls on the mass storage that may be in flight.
// Go cannot cancel a call blocked in openat, and every such call pins an OS thread.
// When the mount hangs, uncapped requests pile up until the runtime dies at 10000
// threads. A healthy read takes milliseconds, so this is far above normal concurrency.
const maxStorageCalls = 256

// ErrBusy is returned instead of waiting when all slots are taken.
var ErrBusy = errors.New("storage busy")

var storageSlots = make(chan struct{}, maxStorageCalls)

var (
	storageCallsInFlight = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "gocast_storage_calls_in_flight",
		Help: "File-system calls on the mass storage currently in flight.",
	})
	storageCallsRejected = promauto.NewCounter(prometheus.CounterOpts{
		Name: "gocast_storage_calls_rejected_total",
		Help: "File-system calls refused because all slots were taken. Any increase means the storage is not answering.",
	})
	_ = promauto.NewGaugeFunc(prometheus.GaugeOpts{
		Name: "gocast_storage_calls_capacity",
		Help: "Number of slots for file-system calls on the mass storage.",
	}, func() float64 { return maxStorageCalls })
)

// withStorageSlot runs fn holding a slot, or fails at once with ErrBusy.
// The slot is held until fn returns, not until a caller gives up: the thread is
// stuck until then.
func withStorageSlot[T any](fn func() (T, error)) (T, error) {
	select {
	case storageSlots <- struct{}{}:
	default:
		storageCallsRejected.Inc()
		var zero T
		return zero, ErrBusy
	}
	storageCallsInFlight.Inc()
	defer func() {
		storageCallsInFlight.Dec()
		<-storageSlots
	}()
	return fn()
}

// ReadFile is os.ReadFile, limited to maxStorageCalls concurrent calls.
func ReadFile(path string) ([]byte, error) {
	return withStorageSlot(func() ([]byte, error) { return os.ReadFile(path) })
}

// Open is os.Open followed by Stat, both under one slot, so neither can pin a thread
// beyond the cap. The slot is released on return: reads from the file are not limited.
func Open(path string) (*os.File, os.FileInfo, error) {
	type opened struct {
		f    *os.File
		info os.FileInfo
	}
	o, err := withStorageSlot(func() (opened, error) {
		f, err := os.Open(path)
		if err != nil {
			return opened{}, err
		}
		info, err := f.Stat()
		if err != nil {
			_ = f.Close()
			return opened{}, err
		}
		return opened{f, info}, nil
	})
	return o.f, o.info, err
}
