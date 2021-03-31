package model

import "gorm.io/gorm"

type Chat struct {
	gorm.Model
	UserID   string
	UserName string
	Message  string
	StreamID uint
}
