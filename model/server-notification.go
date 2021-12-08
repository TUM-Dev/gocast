package model

import (
	"gorm.io/gorm"
	"time"
)

type ServerNotification struct {
	gorm.Model

	Text    string    `gorm:"not null"`
	Warn    bool      `gorm:"not null;default:false"` // if false -> Info
	Start   time.Time `gorm:"not null"`
	Expires time.Time `gorm:"not null"`
}

func (s ServerNotification) FormatFrom() string {
	return s.Start.Format("2006-01-02 15:04")
}

func (s ServerNotification) FormatExpires() string {
	return s.Expires.Format("2006-01-02 15:04")
}
