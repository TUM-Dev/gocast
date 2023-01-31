package model

import (
	"gorm.io/gorm"
)

// Subtitles represents subtitles for a particular stream in a particular language
type Subtitles struct {
	gorm.Model

	StreamID uint   `gorm:"not null"`
	Content  string `gorm:"not null"` // the .srt content provided by the voice-service
	Language string `gorm:"not null"`
}

// TableName returns the name of the table for the Subtitles model in the database.
func (*Subtitles) TableName() string {
	return "subtitles"
}

// BeforeCreate is currently not implemented for Subtitles
func (s *Subtitles) BeforeCreate(tx *gorm.DB) (err error) {
	return nil
}

// AfterFind is currently not implemented for Subtitles
func (s *Subtitles) AfterFind(tx *gorm.DB) (err error) {
	return nil
}
