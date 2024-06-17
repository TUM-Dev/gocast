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
	GetAll(context.Context, uint) ([]model.Runner, error)

	// Get all Runners in an array for a list of a user's administered schools.
	// GetAllForSchools(context.Context, []model.School) ([]model.Runner, error)

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

// Get all Runners in an array for a given school ID.
func (d runnerDao) GetAll(c context.Context, schoolId uint) ([]model.Runner, error) {
	var runners []model.Runner
	err := d.db.WithContext(c).Model(&model.Runner{}).Where("school_id = ?", schoolId).Find(&runners).Error
	if err != nil {
		log.Error("no runners found for school ID: ", schoolId)
		return nil, err
	}
	return runners, nil
}

// Get all Runners in an array for a list of a user's administered schools.
/*func (d runnerDao) GetAllForSchools(c context.Context, schools []model.School) ([]model.Runner, error) {
	var allRunners []model.Runner
	for _, school := range schools {
		var runners []model.Runner
		err := d.db.WithContext(c).Model(&model.Runner{}).Where("school_id = ?", school.ID).Find(&runners).Error
		if err != nil {
			log.Error("no runners found for school ID: ", school.ID)
			return nil, err
		}
		allRunners = append(allRunners, runners...)
	}
	return allRunners, nil
}*/

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
