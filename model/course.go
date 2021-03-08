package model

import (
	"gorm.io/gorm"
)

type Course struct {
	gorm.Model

	UserID              uint
	Name                string
	Slug                string //eg. eidi
	TUMOnlineIdentifier string
	Streams             []Stream
	Students            []Student `gorm:"many2many:course_students;"`
}
