package dao

import (
	"context"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/TUM-Dev/gocast/model"
)

//go:generate go tool mockgen -source=runner.go -destination ../mock_dao/runner.go

type RunnerDao interface {
	// Get Runner by ID
	Get(context.Context, string) (model.Runner, error)

	// Create a new Runner for the database
	Create(context.Context, *model.Runner) error

	// Delete a Runner by hostname.
	Delete(context.Context, string) error

	// Update a Runner by hostname.
	Update(context.Context, *model.Runner) error

	// GetAll gets a list of all Runners.
	GetAll(context.Context) ([]model.Runner, error)

	// GetAvailable returns the runner that currently runs the lease jobs and is not draining
	GetAvailable(context.Context) (model.Runner, error)
}

type runnerDao struct {
	db *gorm.DB
}

func NewRunnerDao() RunnerDao {
	return runnerDao{db: DB}
}

func (d runnerDao) GetAvailable(ctx context.Context) (runner model.Runner, err error) {
	return runner, d.db.WithContext(ctx).Model(model.Runner{}).Order("job_count DESC").Where("draining = 0").Find(&runner).Error
}

// Get a Runner by id.
func (d runnerDao) Get(c context.Context, hostname string) (res model.Runner, err error) {
	return res, d.db.WithContext(c).First(&res, "hostname = ?", hostname).Error
}

// Create a Runner.
func (d runnerDao) Create(c context.Context, it *model.Runner) error {
	return d.db.WithContext(c).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "hostname"}},                                       // key column
		DoUpdates: clause.AssignmentColumns([]string{"port", "version", "time_of_register"}), // column needed to be updated
	}).Create(it).Error
}

// Delete a Runner by hostname.
func (d runnerDao) Delete(c context.Context, hostname string) error {
	return d.db.WithContext(c).Where("hostname = ?", hostname).Delete(&model.Runner{}).Error
}

// Update a Runner
func (d runnerDao) Update(c context.Context, it *model.Runner) error {
	return d.db.WithContext(c).Save(it).Error
}

// GetAll returns all Runners
func (d runnerDao) GetAll(c context.Context) ([]model.Runner, error) {
	var runners []model.Runner
	err := d.db.WithContext(c).Find(&runners).Error
	return runners, err
}
