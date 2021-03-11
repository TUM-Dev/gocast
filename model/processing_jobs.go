package model

import (
	"gorm.io/gorm"
	"time"
)

/**
 * A processing job is inserted after a stream is done.
 * The recording is enqueued to be converted to mp4 and uploaded to the lrz.
 */
type ProcessingJob struct {
	gorm.Model

	FilePath    string `gorm:"unique"` // same file won't be processed twice
	StreamID    uint
	InProgress  bool      `gorm:"default:false"` // set to true once a worker gets assigned to this job.
	AvailableAt time.Time // two hours after lecture ends. Ensures that the lecture was not just paused.
}
