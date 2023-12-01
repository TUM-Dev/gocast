package model

import (
	"context"
	"database/sql"
	"errors"
	"gorm.io/gorm"
	"time"
)

// Runner represents a runner that creates, converts and postprocesses streams and does other heavy lifting.
type Runner struct {
	Hostname string `gorm:"primaryKey"`
	Port     int
	LastSeen sql.NullTime

	Status   string
	Workload uint
	CPU      string
	Memory   string
	Disk     string
	Uptime   string
	Version  string
}

// BeforeCreate returns errors if hostnames and ports of workers are invalid.
func (r *Runner) BeforeCreate(tx *gorm.DB) (err error) {
	if r.Hostname == "" {
		return errors.New("missing hostname")
	}
	if r.Port < 0 || r.Port > 65535 {
		return errors.New("port out of range")
	}
	return nil
}

// SendHeartbeat updates the last seen time of the runner and gives runner stats
func (r *Runner) UpdateStats(tx *gorm.DB, context context.Context) (bool, error) {

	tx.Model(&r).Updates(Runner{
		LastSeen: sql.NullTime{Time: tx.NowFunc(), Valid: true},
		Status:   r.Status,
		Workload: r.Workload,
		CPU:      r.CPU,
		Memory:   r.Memory,
		Disk:     r.Disk,
		Uptime:   r.Uptime,
		Version:  r.Version,
	})

	return true, nil
}

func (r *Runner) isAlive() bool {
	return r.LastSeen.Time.After(time.Now().Add(time.Minute * -6))
}
