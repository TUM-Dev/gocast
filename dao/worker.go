package dao

import (
	"TUM-Live/model"
	"context"
)

func CreateWorker(worker *model.Worker) error {
	return DB.Create(worker).Error
}

func SaveWorker(worker model.Worker) error {
	return DB.Save(&worker).Error
}

func DeleteWorker(workerID string) error {
	return DB.Where("worker_id = ?", workerID).Delete(&model.Worker{}).Error
}

func GetAllWorkers() ([]model.Worker, error) {
	var workers []model.Worker
	err := DB.Find(&workers).Error
	return workers, err
}

// GetAliveWorkers returns all workers that were active within the last 5 minutes
func GetAliveWorkers() []model.Worker {
	var workers []model.Worker
	DB.Model(&model.Worker{}).Where("last_seen > DATE_SUB(NOW(), INTERVAL 5 MINUTE)").Scan(&workers)
	return workers
}

func GetWorkerByHostname(ctx context.Context, hostname string) (model.Worker, error) {
	var worker model.Worker
	err := DB.Where("host = ?", hostname).First(&worker).Error
	return worker, err
}

func GetWorkerByID(ctx context.Context, workerID string) (model.Worker, error) {
	var worker model.Worker
	dbErr := DB.First(&worker, "worker_id = ?", workerID).Error
	return worker, dbErr
}
