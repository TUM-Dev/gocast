package model

import (
	"time"

	"gorm.io/gorm"
)

// ActionStatus represents the current status of an action
type ActionStatus string

const (
	// ActionStatusRunning means the action is currently running
	ActionStatusRunning ActionStatus = "running"
	// ActionStatusCompleted means the action completed successfully
	ActionStatusCompleted ActionStatus = "completed"
	// ActionStatusFailed means the action failed
	ActionStatusFailed ActionStatus = "failed"
)

// Action represents a single action within a job
type Action struct {
	gorm.Model
	// JobID is the ID of the job this action belongs to
	JobID uint `gorm:"column:job_id;not null;index"`
	// ActionType is the type of action (e.g., "stream", "stream_end", "mk_vod", "check_vod", "mk_thumb")
	ActionType string `gorm:"column:action_type;not null"`
	// Status is the current status of the action
	Status ActionStatus `gorm:"column:status;not null;default:'running'"`
	// StartedAt is the timestamp when the action started
	StartedAt *time.Time `gorm:"column:started_at"`
	// CompletedAt is the timestamp when the action completed
	CompletedAt *time.Time `gorm:"column:completed_at"`
	// LastError is the error message if the action failed
	LastError string `gorm:"column:last_error;type:text"`
}

// TableName returns the name of the table for the Action model in the database.
func (*Action) TableName() string {
	return "actions"
}
