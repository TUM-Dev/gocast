package realtimehub

import (
	"context"
	"sync"
	"time"

	protobuf "github.com/TUM-Dev/gocast/apiv2/protobuf/server"
)

const (
	// DefaultBuffer is how many events a subscriber may fall behind by before it is
	// dropped. A chat burst is a few dozen events; a socket further behind than that
	// is better served by a fresh snapshot than by the backlog.
	DefaultBuffer = 64

	// DefaultViewersInterval is the shortest gap between two ViewersEvents for one
	// stream. Joins and leaves arrive in bursts when a lecture starts or ends, and
	// every one would otherwise be an event to every viewer.
	DefaultViewersInterval = 2 * time.Second
)

// Memory is a Hub for a single server process.
type Memory struct {
	mu      sync.Mutex
	streams map[uint]map[*subscriber]struct{}
	viewers map[uint]*viewerThrottle

	buffer          int
	viewersInterval time.Duration
}

type subscriber struct {
	ch     chan *protobuf.RealtimeEvent
	closed bool
}

// viewerThrottle limits a stream's ViewersEvents to one per interval, and makes the
// one that is sent carry the count as of sending rather than as of the change.
type viewerThrottle struct {
	last    time.Time
	pending bool
}

// MemoryOption configures a Memory hub.
type MemoryOption func(*Memory)

// WithBuffer sets how many events a subscriber may fall behind by.
func WithBuffer(n int) MemoryOption {
	return func(m *Memory) { m.buffer = n }
}

// WithViewersInterval sets the shortest gap between two ViewersEvents per stream.
func WithViewersInterval(d time.Duration) MemoryOption {
	return func(m *Memory) { m.viewersInterval = d }
}

// NewMemory returns an empty in-process hub.
func NewMemory(opts ...MemoryOption) *Memory {
	m := &Memory{
		streams:         make(map[uint]map[*subscriber]struct{}),
		viewers:         make(map[uint]*viewerThrottle),
		buffer:          DefaultBuffer,
		viewersInterval: DefaultViewersInterval,
	}
	for _, opt := range opts {
		opt(m)
	}
	if m.buffer < 1 {
		// Dropping hands the subscriber a ResyncEvent, which needs one free slot.
		m.buffer = 1
	}
	return m
}

var _ Hub = (*Memory)(nil)

// Publish hands the event to every subscriber of the stream without waiting for any
// of them. A subscriber whose buffer is full is dropped: its backlog is discarded, it
// receives a ResyncEvent, and its channel is closed. One stalled socket therefore
// costs the others nothing, and the dropped client recovers by reconnecting and
// fetching a snapshot, which it does after every reconnect anyway.
func (m *Memory) Publish(_ context.Context, streamID uint, event *protobuf.RealtimeEvent) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for sub := range m.streams[streamID] {
		select {
		case sub.ch <- event:
		default:
			m.dropLocked(streamID, sub)
		}
	}
}

// dropLocked removes a subscriber that fell behind. Only Publish sends on a
// subscriber's channel, and it holds the lock, so once the backlog is drained the
// ResyncEvent is certain to fit.
func (m *Memory) dropLocked(streamID uint, sub *subscriber) {
	m.removeLocked(streamID, sub)

	for drained := false; !drained; {
		select {
		case <-sub.ch:
		default:
			drained = true
		}
	}
	sub.ch <- ResyncEvent(streamID, "fell behind")
	close(sub.ch)
}

// removeLocked unregisters a subscriber and closes nothing.
func (m *Memory) removeLocked(streamID uint, sub *subscriber) {
	sub.closed = true
	subs := m.streams[streamID]
	delete(subs, sub)
	if len(subs) == 0 {
		delete(m.streams, streamID)
	}
	m.viewersChangedLocked(streamID)
}

// Subscribe implements Hub.
func (m *Memory) Subscribe(streamID uint) (<-chan *protobuf.RealtimeEvent, func()) {
	sub := &subscriber{ch: make(chan *protobuf.RealtimeEvent, m.buffer)}

	m.mu.Lock()
	if m.streams[streamID] == nil {
		m.streams[streamID] = make(map[*subscriber]struct{})
	}
	m.streams[streamID][sub] = struct{}{}
	m.viewersChangedLocked(streamID)
	m.mu.Unlock()

	var once sync.Once
	cancel := func() {
		once.Do(func() {
			m.mu.Lock()
			defer m.mu.Unlock()
			// Already gone if Publish dropped it, and its channel already closed.
			if sub.closed {
				return
			}
			m.removeLocked(streamID, sub)
			close(sub.ch)
		})
	}

	return sub.ch, cancel
}

// Viewers implements Hub.
func (m *Memory) Viewers(streamID uint) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.streams[streamID])
}

// viewersChangedLocked schedules a ViewersEvent for the stream, at most one per
// interval. Always from a timer goroutine, never inline: the caller holds the lock
// that Publish needs.
func (m *Memory) viewersChangedLocked(streamID uint) {
	throttle, ok := m.viewers[streamID]
	if !ok {
		throttle = &viewerThrottle{}
		m.viewers[streamID] = throttle
	}
	if throttle.pending {
		return
	}
	throttle.pending = true

	delay := max(time.Until(throttle.last.Add(m.viewersInterval)), 0)
	time.AfterFunc(delay, func() { m.sendViewers(streamID) })
}

func (m *Memory) sendViewers(streamID uint) {
	m.mu.Lock()
	throttle := m.viewers[streamID]
	throttle.pending = false
	throttle.last = time.Now()
	count := len(m.streams[streamID])
	if count == 0 {
		// Nobody left to tell, and nothing to throttle against when someone comes.
		delete(m.viewers, streamID)
		m.mu.Unlock()
		return
	}
	m.mu.Unlock()

	m.Publish(context.Background(), streamID, ViewersEvent(streamID, count))
}
