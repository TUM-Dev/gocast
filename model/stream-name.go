package model

import (
	"gorm.io/gorm"
	"time"
)

// StreamName is essentially a "streaming slot" used for load balancing
type StreamName struct {
	gorm.Model

	StreamName     string    `gorm:"type:varchar(64); unique; not null"`
	IsTranscoding  bool      `gorm:"not null;default:false"`
	IngestServerID uint      `gorm:"not null"`
	StreamID       uint      // Is null when the slot is not used
	FreedAt        time.Time `gorm:"not null;default:0"`
}
