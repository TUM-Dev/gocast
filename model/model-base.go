package model

import (
	"time"

	"gorm.io/gorm"
)

// Model is a base model that can be embedded in other models
// it's basically the same as gorm.Model but with convenient json annotations
type Model struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	CreatedAt time.Time      `json:"createdAt"`
	UpdatedAt time.Time      `json:"-"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}
