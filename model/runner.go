package model

import (
	"database/sql"
	"errors"
	"gorm.io/gorm"
)

// Runner represents a runner that creates, converts and postprocesses streams and does other heavy lifting.
type Runner struct {
	Hostname string `gorm:"primaryKey"`
	Port     int
	LastSeen sql.NullTime
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
