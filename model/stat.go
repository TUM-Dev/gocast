package model

import (
	"gorm.io/gorm"
	"time"
)

type Stat struct {
	gorm.Model

	Time     time.Time `gorm:"not null"`
	StreamID uint      `gorm:"not null"`
	Viewers  uint      `gorm:"not null"`
	Live     bool      `gorm:"not null"`
}
