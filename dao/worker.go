package dao

import (
	"TUM-Live/model"
	"context"
	"time"
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
	DB.Model(&model.Worker{}).Where("last_seen > ?", time.Now().Add(time.Minute*-5)).Scan(&workers)
	return workers
}

func GetWorkerByID(ctx context.Context, workerID string) (model.Worker, error) {
	var worker model.Worker
	dbErr := DB.First(&worker, "worker_id = ?", workerID).Error
	return worker, dbErr
}
