package dao

import (
	"context"
	"github.com/TUM-Dev/gocast/model"
	"gorm.io/gorm"
)

type JobDao interface {
	CreateJob(ctx context.Context, job model.Job) error
	Get(ctx context.Context, jobID string) (model.Job, error)
	CompleteJob(ctx context.Context, jobID uint32) error
	GetRunners(ctx context.Context, jobID uint32) error
	RemoveAction(ctx context.Context, actionID uint32) error
}

type jobDao struct {
	db *gorm.DB
}

func NewJobDao(db *gorm.DB) JobDao {
	panic("implement me")
}

func (j *jobDao) Get(ctx context.Context, jobID string) (res model.Job, err error) {
	return res, j.db.WithContext(ctx).First(&res, "id = ?", jobID).Error
}

func (j *jobDao) CreateJob(job model.Job) error {
	panic("implement me")
}

func (j *jobDao) CompleteJob(jobID uint32) error {
	panic("implement me")
}

func (j *jobDao) GetRunners(jobID uint32) error {
	panic("implement me")
}

func (j *jobDao) RemoveAction(actionID uint32) error {
	panic("implement me")
}
