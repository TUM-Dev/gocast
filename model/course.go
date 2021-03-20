package model

import (
	"gorm.io/gorm"
)

type Course struct {
	gorm.Model

	UserID              uint
	Name                string
	Slug                string //eg. eidi
	TeachingTerm        string //eg. SoSe2020, WiSe2021
	TUMOnlineIdentifier string
	VODEnabled          bool
	DownloadsEnabled    bool
	ChatEnabled         bool
	Visibility          string //public, loggedin or enrolled
	Streams             []Stream
	Students            []Student `gorm:"many2many:course_students;"`
}
