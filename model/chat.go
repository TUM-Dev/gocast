package model

import (
	"gorm.io/gorm"
	"time"
)

type Chat struct {
	gorm.Model

	UserID   string    `gorm:"not null"`
	UserName string    `gorm:"not null"`
	Message  string    `gorm:"not null"`
	StreamID uint      `gorm:"not null"`
	Admin    bool      `gorm:"not null"`
	SendTime time.Time `gorm:"not null"`
}
