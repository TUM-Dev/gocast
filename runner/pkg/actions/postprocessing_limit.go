package actions

import (
	"context"

	"github.com/tum-dev/gocast/runner/config"
)

// postProcessingSlots bounds the work that is redone for recordings that already exist:
// regenerating a thumbnail and scanning for silences. Redoing a whole course asks for
// hundreds of these at once, each an ffmpeg that reads the recording over HTTP. Run
// together they used up every core of a runner, and it stopped answering altogether,
// recordings in progress included. A job waits for a slot, it does not fail.
//
// A variable so the tests can swap it.
var postProcessingSlots = make(chan struct{}, max(1, config.Config.MaxPostProcessing))

// acquirePostProcessing waits for a slot and returns the func that gives it back, or the
// error of ctx if the job ends first.
func acquirePostProcessing(ctx context.Context) (release func(), err error) {
	slots := postProcessingSlots
	select {
	case slots <- struct{}{}:
		return func() { <-slots }, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
