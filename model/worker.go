package model

import "time"

type Worker struct {
	WorkerID string `gorm:"primaryKey"`
	Host     string // Hostname (e.g., "itovm01")
	Address  string // IP address or FQDN (e.g., worker01.organization.example.com)
	Shared   bool   // Whether the worker can be shared with other organizations
	Ingest   bool   // Whether the worker acts as an ingest worker/server
	Status   string
	Workload uint // How much the worker has to do. +1 per silence detection job, +2 per converting job, +3 per streaming job
	LastSeen time.Time

	// VM stats:
	CPU    string
	Memory string
	Disk   string
	Uptime string

	Version string

	OrganizationID uint `gorm:"not null"` // Organization the worker belongs to
}

func (w *Worker) IsAlive() bool {
	return w.LastSeen.After(time.Now().Add(time.Minute * -6))
}
