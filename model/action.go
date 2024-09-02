package model

import (
	"errors"
	"gorm.io/gorm"
	"time"
)

const (
	completed = iota
	running
	failed
	awaiting
	restarted
)

type Action struct {
	gorm.Model

	ActionID string `gorm:"primaryKey"`

	//foreign keys and references to the job and runner
	AssignedRunner []*Runner `gorm:"foreignKey:Action"`
	Runner         *Runner   `gorm:"foreignKey:Action"`
	Job            *Job      `gorm:"foreignKey:Actions"`

	Type         string `gorm:"not null"`
	Description  string
	assignedDate time.Time
	Start        time.Time
	End          time.Time
	Status       int `gorm:"not null;default:3"`
}

func (a *Action) BeforeCreate(tx *gorm.DB) (err error) {
	if a.Job == nil {
		return errors.New("job needs to be assigned")
	}
	if a.Type == "" {
		return errors.New("type needs to be assigned, unnecessary creation")
	}
	return nil
}

func (a *Action) SetToCompleted() {
	a.Status = completed
}

func (a *Action) SetToRunning() {
	a.Status = running
}

func (a *Action) SetToFailed() {
	a.Status = failed
}

func (a *Action) SetToAwaiting() {
	a.Status = awaiting
}

func (a *Action) SetToRestarted() {
	a.Status = restarted
}

func (a *Action) GetRunner() []*Runner {
	if a.Runner == nil {
		logger.Error("runner not assigned yet")
		return nil
	}
	return a.AssignedRunner
}

func (a *Action) GetJob() *Job {
	if a.Job == nil {
		logger.Error("job not found. Please check database")
		return nil
	}
	return a.Job
}

func (a *Action) GetDescription() string {
	return a.Description
}

func (a *Action) AssignRunner(r *Runner) {
	if r == nil {
		logger.Error("runner not found")
		return
	} else if a.Status != awaiting {
		logger.Error("action already assigned")
		return
	}

	a.AssignedRunner = append(a.AssignedRunner, r)
}

func (a *Action) GetStatus() int {
	return a.Status
}

func (a *Action) GetID() uint {
	return a.ID
}

func (a *Action) GetType() string {
	return a.Type
}

func (a *Action) GetAssignedDate() time.Time {
	return a.assignedDate
}

func (a *Action) GetStart() time.Time {
	return a.Start
}

func (a *Action) GetEnd() time.Time {
	return a.End
}
