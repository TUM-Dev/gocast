package dao

import (
	"context"
	"github.com/joschahenningsen/TUM-Live/model"
	"gorm.io/gorm"
)

//go:generate mockgen -source=worker.go -destination ../mock_dao/worker.go

type WorkerDao interface {
	CreateWorker(worker *model.Worker) error
	SaveWorker(worker model.Worker) error

	GetAllWorkers() ([]model.Worker, error)
	GetAliveWorkers() []model.Worker
	GetWorkerByHostname(ctx context.Context, hostname string) (model.Worker, error)
	GetWorkerByID(ctx context.Context, workerID string) (model.Worker, error)

	DeleteWorker(workerID string) error
}

type workerDao struct {
	db *gorm.DB
}

func NewWorkerDao() WorkerDao {
	return workerDao{db: DB}
}

func (d workerDao) CreateWorker(worker *model.Worker) error {
	return DB.Create(worker).Error
}

func (d workerDao) SaveWorker(worker model.Worker) error {
	return DB.Save(&worker).Error
}

func (d workerDao) GetAllWorkers() ([]model.Worker, error) {
	var workers []model.Worker
	err := DB.Find(&workers).Error
	return workers, err
}

// GetAliveWorkers returns all workers that were active within the last 5 minutes
func (d workerDao) GetAliveWorkers() []model.Worker {
	var workers []model.Worker
	DB.Model(&model.Worker{}).Where("last_seen > DATE_SUB(NOW(), INTERVAL 5 MINUTE)").Scan(&workers)
	return workers
}

func (d workerDao) GetWorkerByHostname(ctx context.Context, hostname string) (model.Worker, error) {
	var worker model.Worker
	err := DB.Where("host = ?", hostname).First(&worker).Error
	return worker, err
}

func (d workerDao) GetWorkerByID(ctx context.Context, workerID string) (model.Worker, error) {
	var worker model.Worker
	dbErr := DB.First(&worker, "worker_id = ?", workerID).Error
	return worker, dbErr
}

func (d workerDao) DeleteWorker(workerID string) error {
	return DB.Where("worker_id = ?", workerID).Delete(&model.Worker{}).Error
}
