package model

import "gorm.io/gorm"

// Job represents todo...
type Job struct {
	gorm.Model

	Actions []Action `gorm:"foreignKey:JobId;"` //

	// todo. Please specify column, type and not null (if required):
	// Name string `gorm:"column:name;type:text;not null;default:'unnamed'"`
}

// TableName returns the name of the table for the Job model in the database.
func (*Job) TableName() string {
	return "job" // todo
}

// BeforeCreate todo
func (j *Job) BeforeCreate(tx *gorm.DB) (err error) {
	return nil
}

// AfterFind todo
func (j *Job) AfterFind(tx *gorm.DB) (err error) {
	return nil
}
