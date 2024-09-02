package model

import (
	"errors"
	"gorm.io/gorm"
	"time"
)

type Job struct {
	gorm.Model

	JobID       string    `gorm:"primaryKey"`
	Actions     []*Action `gorm:"foreignKey:ActionID"`
	Runners     []*Runner `gorm:"foreignKey:Hostname"`
	Description string
	Completed   bool
	Start       time.Time
	End         time.Time
}

func (j *Job) BeforeCreate(tx *gorm.DB) (err error) {
	if j.Actions == nil {
		return errors.New("job has no actions, unnecessary job creation")
	}
	if j.Start.IsZero() || j.End.IsZero() || j.Start.Before(time.Now()) || j.End.After(time.Now()) {
		return errors.New("job has no valid time set. " +
			"Please make sure the time for each start and end value is correct")
	}
	return nil
}

func (j *Job) GetAllActions() ([]*Action, error) {
	if j.Actions == nil {
		return nil, errors.New("no actions found")
	}
	return j.Actions, nil
}

func (j *Job) GetNextAction() (*Action, error) {
	if j.Actions == nil {
		return nil, errors.New("no actions found")
	} else if j.Actions[0].Status == completed {
		return nil, errors.New("action already completed, not pushed")
	}
	action := j.Actions[0]
	j.Actions = j.Actions[1:]
	return action, nil
}

func (j *Job) GetAllRunners() ([]*Runner, error) {
	if j.Runners == nil {
		return nil, errors.New("no actions found")
	}
	return j.Runners, nil
}

func (j *Job) SetToCompleted() error {
	if j.Completed == true {
		return errors.New("job already completed")
	}
	j.Completed = true

	return nil
}

func (j *Job) AddAction(a *Action) error {
	j.Actions = append(j.Actions, a)
	return nil
}
