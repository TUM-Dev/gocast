package actions

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestPostProcessingSlotsBoundConcurrency(t *testing.T) {
	prev := postProcessingSlots
	postProcessingSlots = make(chan struct{}, 2)
	t.Cleanup(func() { postProcessingSlots = prev })

	var running, peak atomic.Int32
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			release, err := acquirePostProcessing(context.Background())
			if err != nil {
				t.Error(err)
				return
			}
			defer release()
			n := running.Add(1)
			for {
				p := peak.Load()
				if n <= p || peak.CompareAndSwap(p, n) {
					break
				}
			}
			time.Sleep(10 * time.Millisecond)
			running.Add(-1)
		}()
	}
	wg.Wait()
	if got := peak.Load(); got != 2 {
		t.Errorf("peak concurrency = %d, want 2", got)
	}
}

func TestAcquirePostProcessingEndsWithTheJob(t *testing.T) {
	prev := postProcessingSlots
	postProcessingSlots = make(chan struct{}, 1)
	t.Cleanup(func() { postProcessingSlots = prev })

	release, err := acquirePostProcessing(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer release()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if _, err := acquirePostProcessing(ctx); err == nil {
		t.Error("acquired a slot that is taken")
	}
}
