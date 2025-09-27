package model

import (
	"gorm.io/gorm"
)

// Action represents todo...
type Action struct {
	gorm.Model

	JobId uint

	// todo. Please specify column, type and not null (if required):
	// Name string `gorm:"column:name;type:text;not null;default:'unnamed'"`
}

// TableName returns the name of the table for the Action model in the database.
func (*Action) TableName() string {
	return "action" // todo
}

// BeforeCreate todo
func (a *Action) BeforeCreate(tx *gorm.DB) (err error) {
	return nil
}

// AfterFind todo
func (a *Action) AfterFind(tx *gorm.DB) (err error) {
	return nil
}
