package dao

import (
	"context"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/TUM-Dev/gocast/model"
)

//go:generate mockgen -source=runner.go -destination ../mock_dao/runner.go

type RunnerDao interface {
	// Get Runner by ID
	Get(context.Context, string) (model.Runner, error)

	// Create a new Runner for the database
	Create(context.Context, *model.Runner) error

	// Delete a Runner by hostname.
	Delete(context.Context, string) error

	// Update a Runner by hostname.
	Update(context.Context, *model.Runner) error
}

type runnerDao struct {
	db *gorm.DB
}

func NewRunnerDao() RunnerDao {
	return runnerDao{db: DB}
}

// Get a Runner by id.
func (d runnerDao) Get(c context.Context, hostname string) (res model.Runner, err error) {
	return res, d.db.WithContext(c).First(&res, "hostname = ?", hostname).Error
}

// Create a Runner.
func (d runnerDao) Create(c context.Context, it *model.Runner) error {
	return d.db.WithContext(c).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "hostname"}},        // key column
		DoUpdates: clause.AssignmentColumns([]string{"port"}), // column needed to be updated
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
