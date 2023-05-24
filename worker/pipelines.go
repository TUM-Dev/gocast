package worker

import (
	"context"
	"github.com/joschahenningsen/TUM-Live/worker/actions"
)

type Pipeline struct {
	Name    string
	Actions []actions.Action
}

func (p *Pipeline) Run(ctx context.Context) (err error) {
	for _, action := range p.Actions {
		ctx, err = action(ctx)
		if err != nil {
			return err
		}
	}
	return err
}

var Pipelines = map[string]*Pipeline{
	"stream-default": {
		"Default Stream Pipeline",
		[]actions.Action{
			actions.StreamAction,
			actions.TranscodeAction,
			actions.UploadAction,
			// ...
		},
	},
}
