package dao

import (
	"TUM-Live/model"
	"context"
	"github.com/getsentry/sentry-go"
	"gorm.io/gorm"
	"time"
)

func SaveWorker(worker model.Worker) {
	err := DB.Save(&worker).Error
	if err != nil {
		sentry.CaptureException(err)
	}
}

func GetAllWorkers() ([]model.Worker, error) {
	var workers []model.Worker
	err := DB.Find(&workers).Error
	return workers, err
}

func GetAliveWorkersOrderedByWorkload() []model.Worker {
	var workers []model.Worker
	DB.Model(&model.Worker{}).Where("last_seen > ?", time.Now().Add(time.Minute*-5)).Order("workload").Scan(&workers)
	return workers
}

func GetWorkerByID(ctx context.Context, workerID string) (model.Worker, error) {
	var worker model.Worker
	dbErr := DB.First(&worker, "worker_id = ?", workerID).Error
	return worker, dbErr
}

func PickJob(ctx context.Context) (job model.ProcessingJob, er error) {
	if Logger != nil {
		Logger(ctx, "Getting a processing job.")
	}
	var foundJob model.ProcessingJob
	err := DB.Transaction(func(tx *gorm.DB) error {
		err := DB.Where("in_progress = 0 AND available_at < NOW()").First(&foundJob).Error
		if err != nil {
			return err
		}
		foundJob.InProgress = true
		DB.Save(&foundJob)
		return nil
	})
	println(foundJob.FilePath)
	return foundJob, err
}
