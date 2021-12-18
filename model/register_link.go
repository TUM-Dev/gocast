package model

import "gorm.io/gorm"

type RegisterLink struct {
	gorm.Model

	UserID         uint   `gorm:"not null"`
	RegisterSecret string `gorm:"not null"`
}
