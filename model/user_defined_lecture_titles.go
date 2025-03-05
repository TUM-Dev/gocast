package model

import (
	"gorm.io/gorm"
)

// UserDefinedLectureTitle represents todo...
type UserDefinedLectureTitle struct {

	// todo. Please specify column, type and not null (if required):
	// Name string `gorm:"column:name;type:text;not null;default:'unnamed'"`
	UserID   string `gorm:"primaryKey" json:"userId"`
	StreamID string `gorm:"primaryKey" json:"streamId"`
	Title    string `gorm:"type:varchar(255)" json:"title"`
}

// TableName returns the name of the table for the UserDefinedLectureTitle model in the database.
func (*UserDefinedLectureTitle) TableName() string {
	return "user_defined_lecture_titles" // todo
}

// BeforeCreate todo
func (u *UserDefinedLectureTitle) BeforeCreate(tx *gorm.DB) (err error) {
	return nil
}

// AfterFind todo
func (u *UserDefinedLectureTitle) AfterFind(tx *gorm.DB) (err error) {
	return nil
}
