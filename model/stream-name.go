package model

import "gorm.io/gorm"

//StreamName is essentially a "streaming slot" used for load balancing
type StreamName struct {
	gorm.Model

	StreamName     string
	IsTranscoding  bool
	IngestServerID uint
	StreamID       uint
}
