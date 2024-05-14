package model

import "gorm.io/gorm"

type RegisterLink struct {
	gorm.Model

	UserID         uint   `gorm:"not null"`
	RegisterSecret string `gorm:"not null"`

	UserOAuthID string `gorm:"not null; type:varchar(50)"` // User's OAuth ID
}
