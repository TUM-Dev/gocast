package model

import (
	"gorm.io/gorm"
)

type VideoSection struct {
	gorm.Model

	Description    string `gorm:"not null" json:"description"`
	StartInSeconds uint   `gorm:"not null" json:"startInSeconds"`
	StreamID       uint   `gorm:"not null" json:"streamID"`
}
