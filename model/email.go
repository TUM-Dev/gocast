package model

import (
	"time"

	"gorm.io/gorm"
)

// Email represents an email to be sent.
type Email struct {
	gorm.Model

	From    string `gorm:"not null"`
	To      string `gorm:"not null"`
	Subject string `gorm:"not null"`
	// A column type needs the `type:` prefix; gorm ignores it otherwise.
	Body    string    `gorm:"type:longtext;not null"`
	Success bool      `gorm:"not null;default:false"`
	Retries int       `gorm:"not null;default:0"`
	LastTry time.Time `gorm:"default:null"`
	// One line per failed attempt, so it needs a text column. No default: a MySQL
	// TEXT column cannot carry one, and asking sized this at varchar(191) — which
	// eleven retries overflowed, making the row unwritable and the send eternal.
	Errors string `gorm:"type:longtext"`
}
