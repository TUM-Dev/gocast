package model

import (
	"gorm.io/gorm"
	"time"
)

type ServerNotification struct {
	gorm.Model

	Text    string
	Warn    bool // if false -> Info
	Start   time.Time
	Expires time.Time
}

func (s ServerNotification) FormatFrom() string {
	return s.Start.Format("2006-01-02 15:04")
}

func (s ServerNotification) FormatExpires() string {
	return s.Expires.Format("2006-01-02 15:04")
}
