package model

import "time"

type Worker struct {
	WorkerID string `gorm:"primaryKey"`
	Host     string
	Status   string
	Workload uint // How much the worker has to do. +1 per silence detection job, +2 per converting job, +3 per streaming job
	LastSeen time.Time
}

func (w *Worker) IsAlive() bool {
	return w.LastSeen.After(time.Now().Add(time.Minute * -6))
}
