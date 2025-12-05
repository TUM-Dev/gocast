package dao

import (
	"context"

	"gorm.io/gorm"

	"github.com/TUM-Dev/gocast/model"
)

//go:generate go tool mockgen -source=job.go -destination ../mock_dao/job.go

// JobDao interface defines methods for job data access
type JobDao interface {
	// Create creates a new job in the database
	Create(context.Context, *model.Job) error

	// Update updates an existing job in the database
	Update(context.Context, *model.Job) error

	// GetByJobID retrieves a job by its unique job ID
	GetByJobID(context.Context, string) (model.Job, error)

	// GetByStreamID retrieves all jobs for a given stream ID
	GetByStreamID(context.Context, uint) ([]model.Job, error)

	// GetByRunnerHostname retrieves all active jobs for a given runner
	GetByRunnerHostname(context.Context, string) ([]model.Job, error)

	// GetActiveJobs retrieves all active jobs (created or running)
	GetActiveJobs(context.Context) ([]model.Job, error)

	// Delete deletes a job by its job ID
	Delete(context.Context, string) error

	// DeleteByStreamID deletes all jobs for a given stream ID
	DeleteByStreamID(context.Context, uint) error

	// CreateAction creates a new action for a job
	CreateAction(context.Context, *model.Action) error

	// UpdateAction updates an existing action
	UpdateAction(context.Context, *model.Action) error

	// GetActionByJobIDAndType retrieves an action by job ID and action type
	GetActionByJobIDAndType(context.Context, uint, string) (model.Action, error)

	// GetActionsByJobID retrieves all actions for a given job ID
	GetActionsByJobID(context.Context, uint) ([]model.Action, error)
}

type jobDao struct {
	db *gorm.DB
}

// NewJobDao creates a new JobDao instance
func NewJobDao() JobDao {
	return jobDao{db: DB}
}

// Create creates a new job in the database
func (d jobDao) Create(ctx context.Context, job *model.Job) error {
	return d.db.WithContext(ctx).Create(job).Error
}

// Update updates an existing job in the database
func (d jobDao) Update(ctx context.Context, job *model.Job) error {
	return d.db.WithContext(ctx).Save(job).Error
}

// GetByJobID retrieves a job by its unique job ID
func (d jobDao) GetByJobID(ctx context.Context, jobID string) (model.Job, error) {
	var job model.Job
	err := d.db.WithContext(ctx).Where("job_id = ?", jobID).First(&job).Error
	return job, err
}

// GetByStreamID retrieves all jobs for a given stream ID
func (d jobDao) GetByStreamID(ctx context.Context, streamID uint) ([]model.Job, error) {
	var jobs []model.Job
	err := d.db.WithContext(ctx).Where("stream_id = ?", streamID).Find(&jobs).Error
	return jobs, err
}

// GetByRunnerHostname retrieves all active jobs for a given runner
func (d jobDao) GetByRunnerHostname(ctx context.Context, hostname string) ([]model.Job, error) {
	var jobs []model.Job
	err := d.db.WithContext(ctx).
		Where("runner_hostname = ? AND status IN ?", hostname, []model.WorkState{model.WorkStateCreated, model.WorkStateRunning}).
		Find(&jobs).Error
	return jobs, err
}

// GetActiveJobs retrieves all active jobs (created or running)
func (d jobDao) GetActiveJobs(ctx context.Context) ([]model.Job, error) {
	var jobs []model.Job
	err := d.db.WithContext(ctx).
		Where("status IN ?", []model.WorkState{model.WorkStateCreated, model.WorkStateRunning}).
		Find(&jobs).Error
	return jobs, err
}

// Delete deletes a job by its job ID
func (d jobDao) Delete(ctx context.Context, jobID string) error {
	return d.db.WithContext(ctx).Where("job_id = ?", jobID).Delete(&model.Job{}).Error
}

// DeleteByStreamID deletes all jobs for a given stream ID
func (d jobDao) DeleteByStreamID(ctx context.Context, streamID uint) error {
	return d.db.WithContext(ctx).Where("stream_id = ?", streamID).Delete(&model.Job{}).Error
}

// CreateAction creates a new action for a job
func (d jobDao) CreateAction(ctx context.Context, action *model.Action) error {
	return d.db.WithContext(ctx).Create(action).Error
}

// UpdateAction updates an existing action
func (d jobDao) UpdateAction(ctx context.Context, action *model.Action) error {
	return d.db.WithContext(ctx).Save(action).Error
}

// GetActionByJobIDAndType retrieves an action by job ID and action type
func (d jobDao) GetActionByJobIDAndType(ctx context.Context, jobID uint, actionType string) (model.Action, error) {
	var action model.Action
	err := d.db.WithContext(ctx).Where("job_id = ? AND action_type = ?", jobID, actionType).First(&action).Error
	return action, err
}

// GetActionsByJobID retrieves all actions for a given job ID
func (d jobDao) GetActionsByJobID(ctx context.Context, jobID uint) ([]model.Action, error) {
	var actions []model.Action
	err := d.db.WithContext(ctx).Where("job_id = ?", jobID).Find(&actions).Error
	return actions, err
}
