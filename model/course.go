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
	VODEnabled          bool
	DownloadsEnabled    bool
	ChatEnabled         bool
	Streams             []Stream
	Recordings          []Recording
	Students            []Student `gorm:"many2many:course_students;"`
}
