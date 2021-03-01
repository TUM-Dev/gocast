package model

import (
	"gorm.io/gorm"
)

// Sessions struct is a row record of the sessions table in the rbglive database
type Session struct {
	gorm.Model

	UserID     uint
	SessionKey string
}
