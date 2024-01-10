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
func (r *Runner) UpdateStats(tx *gorm.DB, ctx context.Context) (bool, error) {
	newStats := ctx.Value("newStats").(Runner)
	err := tx.WithContext(ctx).Model(&r).Updates(newStats).Error
	if err != nil {
		return false, err
	}
	return true, nil
}

func (r *Runner) IsAlive() bool {
	return r.LastSeen.Time.After(time.Now().Add(time.Minute * -6))
}
