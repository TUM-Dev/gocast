package model

import "gorm.io/gorm"

type RegisterLink struct {
	gorm.Model

	UserID           uint
	RegisterSecret string
}
