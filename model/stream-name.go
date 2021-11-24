package model

import "gorm.io/gorm"

//StreamName is essentially a "streaming slot" used for load balancing
type StreamName struct {
	gorm.Model

	StreamName     string `gorm:"not null"`
	IsTranscoding  bool   `gorm:"not null"`
	IngestServerID uint   `gorm:"not null"`
	StreamID       uint   `gorm:"not null"`
}
