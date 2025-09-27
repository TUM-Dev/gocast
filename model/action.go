package model

import (
	"time"

	"gorm.io/gorm"
)

type ActionType int

const (
	ActionStream ActionType = iota
	ActionMkVod
)

// Action represents todo...
type Action struct {
	gorm.Model

	JobId uint

	ActionType ActionType
	StartTime  time.Time `gorm:"default:null"`
	EndTime    time.Time `gorm:"default:null"`
	Failed     bool

	// todo. Please specify column, type and not null (if required):
	// Name string `gorm:"column:name;type:text;not null;default:'unnamed'"`
}

// TableName returns the name of the table for the Action model in the database.
func (*Action) TableName() string {
	return "actions" // todo
}

// BeforeCreate todo
func (a *Action) BeforeCreate(tx *gorm.DB) (err error) {
	return nil
}

// AfterFind todo
func (a *Action) AfterFind(tx *gorm.DB) (err error) {
	return nil
}
