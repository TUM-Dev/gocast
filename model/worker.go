package model

import "time"

type WorkerV2 struct {
	ID       uint `gorm:"primaryKey"`
	Host     string
	Status   string
	Workload uint // How much the worker has to do. +1 per silence detection job, +2 per converting job, +3 per streaming job
	LastSeen time.Time

	// VM stats:
	CPU    string
	Memory string
	Disk   string
	Uptime string

	Version string
}

func (w *WorkerV2) IsAlive() bool {
	return w.LastSeen.After(time.Now().Add(time.Minute * -6))
}
