package model

import (
	"errors"
	"time"
)

type Job struct {
	ID          string
	Actions     []*Action
	Runners     []*Runner
	Description string
	Completed   bool
	Start       time.Time
	End         time.Time
}

func (j Job) GetAllActions() ([]*Action, error) {
	if j.Actions == nil {
		return nil, errors.New("no actions found")
	}
	return j.Actions, nil
}

func (j Job) GetAllRunners() ([]*Runner, error) {
	if j.Runners == nil {
		return nil, errors.New("no actions found")
	}
	return j.Runners, nil
}

func (j Job) SetToCompleted() error {
	if j.Completed == true {
		return errors.New("job already completed")
	}
	j.Completed = true

	return nil
}

func (j Job) AddAction(a *Action) error {
	j.Actions = append(j.Actions, a)
	return nil
}
