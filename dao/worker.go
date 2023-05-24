package dao

import (
	"context"
	"github.com/joschahenningsen/TUM-Live/model"
	"gorm.io/gorm"
)

//go:generate mockgen -source=worker.go -destination ../mock_dao/worker.go

type WorkerDao interface {
	CreateWorker(worker *model.WorkerV2) error
	SaveWorker(worker model.WorkerV2) error

	GetAllWorkers() ([]model.WorkerV2, error)
	GetAliveWorkers() []model.WorkerV2
	GetWorkerByHostname(ctx context.Context, hostname string) (model.WorkerV2, error)
	GetWorkerByID(ctx context.Context, workerID uint) (model.WorkerV2, error)

	DeleteWorker(workerID string) error
}

type workerDao struct {
	db *gorm.DB
}

func NewWorkerDao() WorkerDao {
	return workerDao{db: DB}
}

func (d workerDao) CreateWorker(worker *model.WorkerV2) error {
	return DB.Create(worker).Error
}

func (d workerDao) SaveWorker(worker model.WorkerV2) error {
	return DB.Save(&worker).Error
}

func (d workerDao) GetAllWorkers() ([]model.WorkerV2, error) {
	var workers []model.WorkerV2
	err := DB.Find(&workers).Error
	return workers, err
}

// GetAliveWorkers returns all workers that were active within the last 5 minutes
func (d workerDao) GetAliveWorkers() []model.WorkerV2 {
	var workers []model.WorkerV2
	DB.Model(&model.WorkerV2{}).Where("last_seen > DATE_SUB(NOW(), INTERVAL 5 MINUTE)").Scan(&workers)
	return workers
}

func (d workerDao) GetWorkerByHostname(ctx context.Context, hostname string) (model.WorkerV2, error) {
	var worker model.WorkerV2
	err := DB.Where("host = ?", hostname).First(&worker).Error
	return worker, err
}

func (d workerDao) GetWorkerByID(ctx context.Context, workerID uint) (model.WorkerV2, error) {
	var worker model.WorkerV2
	dbErr := DB.First(&worker, workerID).Error
	return worker, dbErr
}

func (d workerDao) DeleteWorker(workerID string) error {
	return DB.Delete(&model.WorkerV2{}, workerID).Error
}
