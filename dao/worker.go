package dao

import (
	"TUM-Live/model"
	"context"
	"gorm.io/gorm"
	"log"
)

func GetWorkersOrderedByWorkload() []model.Worker {
	var workers []model.Worker
	DB.Model(&model.Worker{}).Order("workload").Scan(&workers)
	return workers
}

func GetWorkerByID(ctx context.Context, workerID string) (model.Worker, error) {
	if Logger != nil {
		Logger(ctx, "Getting worker by id.")
	}
	var worker model.Worker
	dbErr := DB.First(&worker, "worker_id = ?", workerID).Error
	log.Printf("%v", dbErr)
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
