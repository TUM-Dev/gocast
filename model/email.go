package model

import (
	"gorm.io/gorm"
	"time"
)

// Email represents an email to be sent.
type Email struct {
	gorm.Model

	From    string    `gorm:"not null"`
	To      string    `gorm:"not null"`
	Subject string    `gorm:"not null"`
	Body    string    `gorm:"longtext;not null"`
	Success bool      `gorm:"not null;default:false"`
	Retries int       `gorm:"not null;default:0"`
	LastTry time.Time `gorm:"default:null"`
	Errors  string    `gorm:"longtext;default:null"`
}
