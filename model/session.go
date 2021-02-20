package model

import (
	"gorm.io/gorm"
	"time"
)


// Sessions struct is a row record of the sessions table in the rbglive database
type Session struct {
	gorm.Model
	ID int
	Created time.Time
	SessionID string
	UserID int
	User User `gorm:"constraint:OnUpdate:CASCADE,OnDelete:SET NULL;"`
}
