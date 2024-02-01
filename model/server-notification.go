package model

import (
	"errors"
	"html/template"
	"time"

	"gorm.io/gorm"
)

// ServerNotification todo: rename to ServerAlert to avoid confusion with Notification
type ServerNotification struct {
	gorm.Model

	Text    string    `gorm:"not null"`
	Warn    bool      `gorm:"not null;default:false"` // if false -> Info
	Start   time.Time `gorm:"not null"`
	Expires time.Time `gorm:"not null"`
}

func (s *ServerNotification) BeforeCreate(tx *gorm.DB) (err error) {
	if s.Expires.Before(s.Start) {
		err = errors.New("can't save notification where expires is before start")
	}
	return
}

func (s *ServerNotification) FormatFrom() string {
	return s.Start.Format("2006-01-02 15:04")
}

func (s *ServerNotification) FormatExpires() string {
	return s.Expires.Format("2006-01-02 15:04")
}

func (s *ServerNotification) HTML() template.HTML {
	return template.HTML(s.Text)
}
