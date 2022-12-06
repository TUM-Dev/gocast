package model

import (
	"fmt"
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

func (b Bookmark) TimestampAsString() string {
	if b.Hours == 0 {
		return fmt.Sprintf("%02d:%02d", b.Minutes, b.Seconds)
	}
	return fmt.Sprintf("%02d:%02d:%02d", b.Hours, b.Minutes, b.Seconds)
}
