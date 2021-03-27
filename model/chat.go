package model

import "gorm.io/gorm"

type Chat struct {
	gorm.Model
	UserID   string
	Message  string
	StreamID uint
}
