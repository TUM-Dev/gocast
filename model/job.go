package model

import (
	"time"

	"gorm.io/gorm"
)

// JobStatus represents the current status of a job
type JobStatus string

const (
	// JobStatusCreated means the job was just created
	JobStatusCreated JobStatus = "created"
	// JobStatusRunning means the job is currently running
	JobStatusRunning JobStatus = "running"
	// JobStatusCompleted means the job completed successfully
	JobStatusCompleted JobStatus = "completed"
	// JobStatusFailed means the job failed
	JobStatusFailed JobStatus = "failed"
	// JobStatusCancelled means the job was cancelled
	JobStatusCancelled JobStatus = "cancelled"
)

// Job represents a runner job that processes stream-related tasks.
// Jobs are created when a stream request is made to a runner,
// and track the progress through multiple actions.
type Job struct {
	gorm.Model
	// JobID is the unique identifier for the job (UUID from runner)
	JobID string `gorm:"column:job_id;uniqueIndex;not null"`
	// RunnerHostname is the hostname of the runner executing the job
	RunnerHostname string `gorm:"column:runner_hostname;not null;index"`
	// StreamID is the ID of the stream this job is associated with
	StreamID uint `gorm:"column:stream_id;not null;index"`
	// StreamVersion is the version of the stream (COMB, CAM, PRES)
	StreamVersion StreamVersion `gorm:"column:stream_version;not null"`
	// Status is the current status of the job
	Status JobStatus `gorm:"column:status;not null;default:'created';index"`
	// StartedAt is the timestamp when the job started
	StartedAt *time.Time `gorm:"column:started_at"`
	// CompletedAt is the timestamp when the job completed (or failed)
	CompletedAt *time.Time `gorm:"column:completed_at"`
	// Actions is the list of actions executed as part of this job
	Actions []Action `gorm:"foreignKey:JobID;references:ID"`
}

// TableName returns the name of the table for the Job model in the database.
func (*Job) TableName() string {
	return "jobs"
}

// IsActive returns true if the job is still active (not completed, failed, or cancelled)
func (j *Job) IsActive() bool {
	return j.Status == JobStatusCreated || j.Status == JobStatusRunning
}
