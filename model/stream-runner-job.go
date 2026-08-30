package model

import "gorm.io/gorm"

// StreamRunnerJob tracks which runner is executing which job for a given stream version,
// so the job can later be canceled (e.g. when an admin manually ends the stream).
type StreamRunnerJob struct {
	gorm.Model

	StreamID       uint          `gorm:"not null;index"`
	Version        StreamVersion `gorm:"not null"`
	RunnerHostname string        `gorm:"not null"`
	JobID          string        `gorm:"not null"`
}
