package model

import "gorm.io/gorm"

type Silence struct {
	gorm.Model `json:"omitempty"`

	Start    uint `json:"start"`
	End      uint `json:"end"`
	StreamID uint `json:"stream_id,omitempty"`
}
