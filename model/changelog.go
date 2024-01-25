package model

import (
	"gorm.io/gorm"
)

type Changelog struct {
	gorm.Model

	Version string `gorm:"not null"`
	Text    string `gorm:"not null"`
}
