package model

import "gorm.io/gorm"

//StreamName is essentially a "streaming slot" used for load balancing
type StreamName struct {
	gorm.Model

	StreamName     string `gorm:"unique;not null"`
	IsTranscoding  bool   `gorm:"not null;default:false"`
	IngestServerID uint   `gorm:"not null"`
	StreamID       uint   // Is null when the slot is not used
}
