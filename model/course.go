package model

import (
	"gorm.io/gorm"
	"time"
)

type Course struct {
	gorm.Model

	UserID              uint
	Name                string
	Slug                string //eg. eidi
	Year                int // eg. 2021
	TeachingTerm        string //eg. Summer/Winter
	TUMOnlineIdentifier string
	VODEnabled          bool
	DownloadsEnabled    bool
	ChatEnabled         bool
	Visibility          string //public, loggedin or enrolled
	Streams             []Stream
	Students            []Student `gorm:"many2many:course_students;"`
}

func (c Course) GetNextLectureDate() time.Time {
	n := time.Now()
	for _, s := range c.Streams {
		if s.Start.After(n) {
			return s.Start
		}
	}
	return time.Now()
}

func (c Course) HasNextLecture() bool {
	n := time.Now()
	for _, s := range c.Streams {
		if s.Start.After(n) {
			return true
		}
	}
	return false
}

func (c Course) GetRecordings() []Stream {
	var recordings []Stream
	for _, s := range c.Streams {
		if s.Recording {
			recordings = append(recordings, s)
		}
	}
	return recordings
}
