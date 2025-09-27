package dao

import (
	"context"
	"github.com/TUM-Dev/gocast/model"
	"gorm.io/gorm"
)

//go:generate mockgen -source=job.go -destination ../mock_dao/job.go

type JobDao interface {
	// Get Job by ID
	Get(context.Context, uint) (model.Job, error)

	// Create a new Job for the database
	Create(context.Context, *model.Job) error

	// Delete a Job by id.
	Delete(context.Context, uint) error
}

type jobDao struct {
	db *gorm.DB
}

func NewJobDao() JobDao {
	return jobDao{db: DB}
}

// Get a Job by id.
func (d jobDao) Get(c context.Context, id uint) (res model.Job, err error) {
	return res, d.db.WithContext(c).First(&res, id).Error
}

// Create a Job.
func (d jobDao) Create(c context.Context, it *model.Job) error {
	return d.db.WithContext(c).Create(it).Error
}

// Delete a Job by id.
func (d jobDao) Delete(c context.Context, id uint) error {
	return d.db.WithContext(c).Delete(&model.Job{}, id).Error
}
