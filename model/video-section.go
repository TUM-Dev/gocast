package model

import (
	"fmt"
	"gorm.io/gorm"
)

type VideoSection struct {
	gorm.Model

	Description  string `gorm:"not null;index:,class:FULLTEXT" json:"description"`
	StartHours   uint   `gorm:"not null" json:"startHours"`
	StartMinutes uint   `gorm:"not null" json:"startMinutes"`
	StartSeconds uint   `gorm:"not null" json:"startSeconds"`

	StreamID uint `gorm:"not null" json:"streamID"`
	FileID   uint `gorm:"not null" json:"fileID"`
}

func (v VideoSection) TimestampAsString() string {
	if v.StartHours == 0 {
		return fmt.Sprintf("%02d:%02d", v.StartMinutes, v.StartSeconds)
	}
	return fmt.Sprintf("%02d:%02d:%02d", v.StartHours, v.StartMinutes, v.StartSeconds)
}
