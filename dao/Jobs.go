package dao

import (
	"context"
	"github.com/TUM-Dev/gocast/model"
	"gorm.io/gorm"
)

//go:generate mockgen -source=Jobs.go -destination ../mock_dao/jobs.go

type JobDao interface {
	CreateJob(ctx context.Context, job model.Job) error
	Get(ctx context.Context, jobID string) (model.Job, error)
	CompleteJob(ctx context.Context, jobID uint32) error
	GetRunners(ctx context.Context, jobID uint32) error
	RemoveAction(ctx context.Context, actionID uint32) error
	GetAllOpenJobs(ctx context.Context) ([]model.Job, error)
}

type jobDao struct {
	db *gorm.DB
}

func NewJobDao() JobDao {
	return jobDao{db: DB}
}

func (j jobDao) Get(ctx context.Context, jobID string) (res model.Job, err error) {
	return res, j.db.WithContext(ctx).First(&res, "id = ?", jobID).Error
}

func (j jobDao) CreateJob(ctx context.Context, job model.Job) error {
	panic("implement me")
}

func (j jobDao) CompleteJob(ctx context.Context, jobID uint32) error {
	panic("implement me")
}

func (j jobDao) GetRunners(ctx context.Context, jobID uint32) error {
	panic("implement me")
}

func (j jobDao) RemoveAction(ctx context.Context, actionID uint32) error {
	panic("implement me")
}

func (j jobDao) GetAllOpenJobs(ctx context.Context) ([]model.Job, error) {
	var jobs []model.Job
	err := j.db.WithContext(ctx).Model(&model.Job{}).Find(&jobs).Where("completed = ?", false).Error
	return jobs, err
}
