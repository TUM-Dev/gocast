package model

import (
	"gorm.io/gorm"
	"time"
)

// Streams struct is a row record of the streams table in the rbglive database
type Stream struct {
	gorm.Model

	CourseID    uint
	Start       time.Time
	End         time.Time
	StreamKey   string
	VodEnabled  bool
	PlaylistUrl string
	LiveNow     bool
}
