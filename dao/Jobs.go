package dao

import (
	"github.com/TUM-Dev/gocast/model"
	"gorm.io/gorm"
)

type JobDao interface {
	CreateJob(job model.Job) error
	CompleteJob(jobID uint32) error
	GetRunners(jobID uint32) error
	RemoveAction(actionID uint32) error
}

type jobDao struct {
	db *gorm.DB
}
