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
	CompleteJob(ctx context.Context, job model.Job) error
	GetRunners(ctx context.Context, job model.Job) ([]*model.Runner, error)
	RemoveAction(ctx context.Context, job model.Job, actionID uint32) error
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
	return j.db.WithContext(ctx).Create(&job).Error
}

func (j jobDao) CompleteJob(ctx context.Context, job model.Job) error {
	return j.db.WithContext(ctx).Model(&job).Update("complete", true).Error
}

func (j jobDao) GetRunners(ctx context.Context, job model.Job) ([]*model.Runner, error) {
	var runners []*model.Runner
	err := j.db.WithContext(ctx).Model(&job).Association("JobRunner").Find(&runners)
	return runners, err
}

func (j jobDao) RemoveAction(ctx context.Context, job model.Job, actionID uint32) error {
	return j.db.WithContext(ctx).Delete(&model.Action{}, "job_id = ? AND action_id = ?", job.ID, actionID).Error

}

func (j jobDao) GetAllOpenJobs(ctx context.Context) ([]model.Job, error) {
	var jobs []model.Job
	err := j.db.WithContext(ctx).Model(&model.Job{}).Preload("Actions").Find(&jobs).Where("completed = ?", false).Error
	return jobs, err
}
