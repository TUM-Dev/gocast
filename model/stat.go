package model

import (
	"gorm.io/gorm"
	"time"
)

type Stat struct {
	gorm.Model

	Time     time.Time
	StreamID uint
	Viewers  int
	Live     bool
}
