package dao

import (
	"context"
	"github.com/TUM-Dev/gocast/model"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

//go:generate mockgen -source=runner.go -destination ../mock_dao/runner.go

type RunnerDao interface {
	// Get Runner by hostname
	Get(context.Context, string) (model.Runner, error)

	// Get all Runners in an array
	GetAll(context.Context) ([]model.Runner, error)

	// Create a new Runner for the database
	Create(context.Context, *model.Runner) error

	// Delete a Runner by hostname.
	Delete(context.Context, string) error
}

type runnerDao struct {
	db *gorm.DB
}

func NewRunnerDao() RunnerDao {
	return runnerDao{db: DB}
}

// Get a Runner by id.
func (d runnerDao) Get(c context.Context, hostname string) (res model.Runner, err error) {
	return res, DB.WithContext(c).First(&res, "hostname = ?", hostname).Error
}

// Get all Runners in an array
func (d runnerDao) GetAll(c context.Context) ([]model.Runner, error) {
	var runners []model.Runner
	err := d.db.WithContext(c).Model(&model.Runner{}).Find(&runners).Error
	if err != nil {
		log.Error("no runners found")
		return nil, err
	}
	return runners, err
}

// Create a Runner.
func (d runnerDao) Create(c context.Context, it *model.Runner) error {
	return DB.WithContext(c).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "hostname"}},
		UpdateAll: true,
	}).Create(&it).Error
}

// Delete a Runner by hostname.
func (d runnerDao) Delete(c context.Context, hostname string) error {
	return DB.WithContext(c).Delete(&model.Runner{}, hostname).Error
}
