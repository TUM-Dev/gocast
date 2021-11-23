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

func GetAllWorkers() ([]model.Worker, error) {
	var workers []model.Worker
	err := DB.Find(&workers).Error
	return workers, err
}

func GetAliveWorkers() []model.Worker {
	var workers []model.Worker
	DB.Model(&model.Worker{}).Where("last_seen > DATEADD(minute, -5, start)").Scan(&workers)
	return workers
}

func GetWorkerByID(ctx context.Context, workerID string) (model.Worker, error) {
	var worker model.Worker
	dbErr := DB.First(&worker, "worker_id = ?", workerID).Error
	return worker, dbErr
}
