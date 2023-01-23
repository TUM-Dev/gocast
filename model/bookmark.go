package model

import (
	"gorm.io/gorm"
)

type Bookmark struct {
	gorm.Model

	Description string `gorm:"not null" json:"description"`
	Hours       uint   `gorm:"not null" json:"hours"`
	Minutes     uint   `gorm:"not null" json:"minutes"`
	Seconds     uint   `gorm:"not null" json:"seconds"`
	UserID      uint   `gorm:"not null" json:"-"`
	StreamID    uint   `gorm:"not null" json:"-"`
}
