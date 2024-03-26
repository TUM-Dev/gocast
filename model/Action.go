package model

import "time"

type Action struct {
	ID          string
	Type        string
	Runner      *Runner
	Job         *Job
	Description string
	Start       time.Time
	End         time.Time
	Completed   bool
	Workload    int
}

func (a Action) SetToCompleted() {
	a.Completed = true
}
