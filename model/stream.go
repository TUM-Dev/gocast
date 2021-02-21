package model

import (
	"gorm.io/gorm"
	"time"
)

// Streams struct is a row record of the streams table in the rbglive database
type Stream struct {
	gorm.Model
	ID         int
	Start      time.Time
	End        time.Time
	StreamKey  string
	CourseID   int
	Course     Course `gorm:"constraint:OnUpdate:CASCADE,OnDelete:SET NULL;"`
	VodEnabled bool
}
