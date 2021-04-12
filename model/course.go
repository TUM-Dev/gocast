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
	Year                int    // eg. 2021
	TeachingTerm        string //eg. Summer/Winter
	TUMOnlineIdentifier string
	VODEnabled          bool
	DownloadsEnabled    bool
	ChatEnabled         bool
	Visibility          string //public, loggedin or enrolled
	Streams             []Stream
	Students            []Student `gorm:"many2many:course_students;"`
	Users               []User    `gorm:"many2many:course_users;"`
}

func (c Course) GetNextLectureDate() time.Time {
	earliestLecture := time.Now().Add(time.Hour * 24 * 365 * 10) // 10 years from now.
	for _, s := range c.Streams {
		if s.Start.Before(earliestLecture) && s.End.After(time.Now()) {
			earliestLecture = s.Start
		}
	}
	return earliestLecture
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
