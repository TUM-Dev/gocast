package model

import (
	"gorm.io/gorm"
	"time"
)

type Recording struct {
	gorm.Model
	Name        string
	CourseID    uint
	Start       time.Time
	PlaylistUrl string
}