package model

import (
	"errors"

	"gorm.io/gorm"
)

// UserDefinedLectureTitle represents a custom lecture title for a stream by one user
type UserDefinedLectureTitle struct {
	UserID   uint   `gorm:"primaryKey" json:"userId"`
	StreamID uint   `gorm:"primaryKey" json:"streamId"`
	Title    string `gorm:"type:varchar(256)" json:"title"`
}

// BeforeCreate is a GORM hook that is called before a new user is created.
// UserDefinedLectureTitle will not be saved if the title is too long
func (u *UserDefinedLectureTitle) BeforeCreate(tx *gorm.DB) (err error) {
	if len(u.Title) > 256 {
		return errors.New("title is too long")
	}
	return nil
}
