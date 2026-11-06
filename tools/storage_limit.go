package tools

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

// ErrStorageBusy is returned instead of waiting when all slots are taken.
var ErrStorageBusy = errors.New("storage busy")

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

// withStorageSlot runs fn holding a slot, or fails at once with ErrStorageBusy.
// The slot is held until fn returns, not until a caller gives up: the thread is
// stuck until then.
func withStorageSlot[T any](fn func() (T, error)) (T, error) {
	select {
	case storageSlots <- struct{}{}:
	default:
		storageCallsRejected.Inc()
		var zero T
		return zero, ErrStorageBusy
	}
	storageCallsInFlight.Inc()
	defer func() {
		storageCallsInFlight.Dec()
		<-storageSlots
	}()
	return fn()
}

// ReadStorageFile is os.ReadFile, limited to maxStorageCalls concurrent calls.
func ReadStorageFile(path string) ([]byte, error) {
	return withStorageSlot(func() ([]byte, error) { return os.ReadFile(path) })
}

// OpenStorageFile is os.Open, limited to maxStorageCalls concurrent calls.
// Only the open is limited, not the reads from the returned file.
func OpenStorageFile(path string) (*os.File, error) {
	return withStorageSlot(func() (*os.File, error) { return os.Open(path) })
}
