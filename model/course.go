package model

import (
	"gorm.io/gorm"
	"time"
)

type Course struct {
	gorm.Model

	ID int
	Name string
	Start time.Time
	End time.Time
	Semester string
}

