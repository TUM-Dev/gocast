package model

import (
	"gorm.io/gorm"
	"time"
)

type Stream struct {
	gorm.Model

	Name        string
	CourseID    uint
	Start       time.Time
	End         time.Time
	StreamKey   string
	PlaylistUrl string
	LiveNow     bool
}
