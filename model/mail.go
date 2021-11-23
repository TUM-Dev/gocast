package model

import "gorm.io/gorm"

type Mail struct {
	gorm.Model

	To   string `gorm:"not null;"`
	Body string `gorm:"not null;"`
}
