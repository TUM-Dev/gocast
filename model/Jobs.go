package model

import "time"

type Job struct {
	ID          uint32
	Actions     []*Action
	Runners     []*Runner
	Description string
	Completed   bool
	Start       time.Time
	End         time.Time
}

func (j Job) GetAllActions() ([]*Action, error) {
	panic("implement me")
}

func (j Job) GetAllRunners() ([]*Runner, error) {
	panic("implement me")
}
