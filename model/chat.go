package model

import (
	"gorm.io/gorm"
	"time"
)

type Chat struct {
	gorm.Model

	UserID   string
	UserName string
	Message  string
	StreamID uint
	Admin    bool
	SendTime time.Time
}
