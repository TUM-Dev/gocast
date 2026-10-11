package realtimehub

import (
	"context"
	"sync"
	"testing"
	"time"

	protobuf "github.com/TUM-Dev/gocast/apiv2/protobuf/server"
)

// receive returns the next event, failing the test if none arrives promptly.
func receive(t *testing.T, ch <-chan *protobuf.RealtimeEvent) *protobuf.RealtimeEvent {
	t.Helper()
	select {
	case ev, ok := <-ch:
		if !ok {
			t.Fatal("channel closed, wanted an event")
		}
		return ev
	case <-time.After(time.Second):
		t.Fatal("no event within a second")
		return nil
	}
}

// receiveSkippingViewers returns the next event that is not a ViewersEvent, which
// the hub sends on its own schedule whenever someone subscribes.
func receiveSkippingViewers(t *testing.T, ch <-chan *protobuf.RealtimeEvent) *protobuf.RealtimeEvent {
	t.Helper()
	for {
		if ev := receive(t, ch); ev.GetViewers() == nil {
			return ev
		}
	}
}

func TestPublishReachesOnlyTheStreamsSubscribers(t *testing.T) {
	hub := NewMemory(WithViewersInterval(time.Hour))

	a, cancelA := hub.Subscribe(1)
	defer cancelA()
	b, cancelB := hub.Subscribe(1)
	defer cancelB()
	other, cancelOther := hub.Subscribe(2)
	defer cancelOther()

	hub.Publish(context.Background(), 1, TitleEvent(1, "Hopfen"))

	for _, ch := range []<-chan *protobuf.RealtimeEvent{a, b} {
		if got := receiveSkippingViewers(t, ch).GetTitle().GetTitle(); got != "Hopfen" {
			t.Errorf("title = %q, want Hopfen", got)
		}
	}

	// Drain stream 2's own ViewersEvent; nothing else may follow.
	receive(t, other)
	select {
	case ev := <-other:
		t.Fatalf("stream 2 received stream 1's event: %v", ev)
	case <-time.After(50 * time.Millisecond):
	}
}

func TestCancelClosesTheChannelAndIsIdempotent(t *testing.T) {
	hub := NewMemory(WithViewersInterval(time.Hour))
	ch, cancel := hub.Subscribe(1)

	cancel()
	cancel()

	for range ch {
		// Drains a ViewersEvent sent before the cancel, if the timer won the race.
	}
	if n := hub.Viewers(1); n != 0 {
		t.Errorf("Viewers = %d after cancel, want 0", n)
	}

	// Publishing to a stream with nobody on it is a no-op, not a panic.
	hub.Publish(context.Background(), 1, TitleEvent(1, "x"))
}

func TestSlowSubscriberIsDroppedWithAResync(t *testing.T) {
	hub := NewMemory(WithBuffer(2), WithViewersInterval(time.Hour))

	slow, cancelSlow := hub.Subscribe(1)
	defer cancelSlow()
	fast, cancelFast := hub.Subscribe(1)
	defer cancelFast()

	// Let the subscription's ViewersEvent land first, so it is not what overflows.
	time.Sleep(20 * time.Millisecond)

	var wg sync.WaitGroup
	wg.Add(1)
	var fastGot []string
	go func() {
		defer wg.Done()
		for ev := range fast {
			if title := ev.GetTitle(); title != nil {
				fastGot = append(fastGot, title.GetTitle())
				if len(fastGot) == 5 {
					return
				}
			}
		}
	}()

	for _, title := range []string{"1", "2", "3", "4", "5"} {
		hub.Publish(context.Background(), 1, TitleEvent(1, title))
		// The fast reader keeps up; the slow one never reads.
		time.Sleep(5 * time.Millisecond)
	}
	wg.Wait()

	if len(fastGot) != 5 {
		t.Fatalf("fast subscriber got %v, want all five", fastGot)
	}

	var last *protobuf.RealtimeEvent
	for ev := range slow { // closed by the hub, so this terminates
		last = ev
	}
	if last.GetResync() == nil {
		t.Fatalf("slow subscriber's last event = %v, want a resync", last)
	}
	if n := hub.Viewers(1); n != 1 {
		t.Errorf("Viewers = %d after the drop, want 1", n)
	}

	// The dropped subscriber's cancel must not close its channel a second time.
	cancelSlow()
}

func TestViewersEventsAreThrottled(t *testing.T) {
	const interval = 100 * time.Millisecond
	hub := NewMemory(WithViewersInterval(interval))

	watcher, cancel := hub.Subscribe(1)
	defer cancel()

	if got := receive(t, watcher).GetViewers().GetViewers(); got != 1 {
		t.Fatalf("first ViewersEvent = %d, want 1", got)
	}

	start := time.Now()
	var cancels []func()
	for range 3 {
		_, c := hub.Subscribe(1)
		cancels = append(cancels, c)
	}
	defer func() {
		for _, c := range cancels {
			c()
		}
	}()

	// Three joins, one event, carrying the count once they had all happened.
	ev := receive(t, watcher)
	if got := ev.GetViewers().GetViewers(); got != 4 {
		t.Errorf("ViewersEvent = %d, want 4", got)
	}
	if elapsed := time.Since(start); elapsed < interval/2 {
		t.Errorf("second ViewersEvent after %v, want it held back by the interval", elapsed)
	}

	select {
	case extra := <-watcher:
		t.Errorf("unexpected extra event %v", extra)
	case <-time.After(2 * interval):
	}

	if n := hub.Viewers(1); n != 4 {
		t.Errorf("Viewers = %d, want 4", n)
	}
}

func TestConcurrentUse(t *testing.T) {
	hub := NewMemory(WithBuffer(4), WithViewersInterval(time.Millisecond))

	var wg sync.WaitGroup
	for i := range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ch, cancel := hub.Subscribe(uint(i % 3))
			go func() {
				for range ch {
				}
			}()
			for j := range 20 {
				hub.Publish(context.Background(), uint(j%3), TitleEvent(uint(j%3), "t"))
			}
			cancel()
		}()
	}
	wg.Wait()

	for id := uint(0); id < 3; id++ {
		if n := hub.Viewers(id); n != 0 {
			t.Errorf("stream %d has %d viewers after everyone left", id, n)
		}
	}
}

func TestEventEnvelope(t *testing.T) {
	ev := LiveEvent(7, true)
	if ev.GetStreamId() != 7 || ev.GetAt() == nil || !ev.GetLive().GetLive() {
		t.Errorf("LiveEvent = %v", ev)
	}
	// The zero audience delivers to nobody, so every constructor has to set one.
	if ev.GetAudience() != protobuf.RealtimeEvent_AUDIENCE_ALL {
		t.Errorf("audience = %v, want ALL", ev.GetAudience())
	}
}
