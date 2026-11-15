package model

import (
	"time"

	"gorm.io/gorm"
)

type Stat struct {
	gorm.Model

	Time     time.Time `gorm:"not null;index:idx_stats_stream_live_time,priority:3"`
	StreamID uint      `gorm:"not null;index:idx_stats_stream_live_time,priority:1"`
	Viewers  uint      `gorm:"not null;default:0"`
	Live     bool      `gorm:"not null;default:false;index:idx_stats_stream_live_time,priority:2"`
}
