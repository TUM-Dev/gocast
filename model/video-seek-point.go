package model

import (
	"gorm.io/gorm"
)

type VideoSeekPoint struct {
	gorm.Model
	SeekPosition float64 `gorm:"not null" json:"seekPosition"`
	StreamID     uint    `gorm:"not null" json:"streamID"`
}
