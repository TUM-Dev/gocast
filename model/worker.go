package model

import "time"

type Worker struct {
	WorkerID string `gorm:"primaryKey"`
	Host     string
	Status   string
	Workload int // How much the worker has to do. +1 per silence detection job, +2 per converting job, +3 per streaming job
	LastSeen time.Time
}
