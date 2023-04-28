package worker

import "context"

type streamAction struct {
}

func (a streamAction) Do() error {
	// todo stream
	return nil
}

type transcodeAction struct {
}

func (a transcodeAction) Do() error {
	// todo transcode
	return nil
}

var postprocessPipeline = &pipeline{
	weight:  1,
	actions: []Action{
		// todo
	},
}

var transcodePipeline = &pipeline{
	weight: 1,
	actions: []Action{
		&transcodeAction{},
	},
	runNextOnErr: false,
	next:         postprocessPipeline,
}

var streamPipeline = &pipeline{
	weight: 2,
	actions: []Action{
		&streamAction{},
	},
	runNextOnErr: false,
	next:         transcodePipeline,
}

type pipeline struct {
	c context.Context // use context to cancel pipeline

	weight int

	actions []Action
	run     func() error

	next         *pipeline
	runNextOnErr bool
}

type Action interface {
	Do() error
}
