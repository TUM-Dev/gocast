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

	AllRunners   []Runner `gorm:"many2many:action_runners;"`
	JobID        uint
	Type         string `gorm:"not null"`
	Description  string
	assignedDate time.Time
	Start        time.Time
	End          time.Time
	Status       int `gorm:"not null;default:3"`
}

func (a *Action) BeforeCreate(*gorm.DB) (err error) {
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

func (a *Action) GetDescription() string {
	return a.Description
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
