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

	// ReserveRunner returns the runner that currently runs the least jobs and is not draining.
	// It also increments the number of jobs assigned to the runner.
	ReserveRunner(context.Context) (model.Runner, error)
}

type runnerDao struct {
	db *gorm.DB
}

func NewRunnerDao() RunnerDao {
	return runnerDao{db: DB}
}

func (d runnerDao) ReserveRunner(ctx context.Context) (runner model.Runner, err error) {
	tx := d.db.WithContext(ctx).Begin()
	if tx.Error != nil {
		return runner, tx.Error
	}

	// 1. Select the least-loaded runner, and LOCK the selected row(s) for update
	err = tx.Model(&model.Runner{}).
		Where("draining = ? AND last_seen > DATE_SUB(NOW(), INTERVAL 20 SECOND)", 0).
		Order("job_count ASC").
		Limit(1).
		Set("gorm:query_option", "FOR UPDATE"). // Lock the selected row
		Find(&runner).Error
	if err != nil {
		tx.Rollback()
		return runner, err
	}

	err = tx.Model(&runner).UpdateColumn("job_count", gorm.Expr("job_count + ?", 1)).Error
	if err != nil {
		tx.Rollback()
		return runner, err
	}

	if err = tx.Commit().Error; err != nil {
		return runner, err
	}

	return runner, nil
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
