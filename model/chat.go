package model

import "gorm.io/gorm"

type Chat struct {
	gorm.Model
	UserID   uint
	Message  string
	StreamID uint
}
