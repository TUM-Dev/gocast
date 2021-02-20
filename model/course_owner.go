package model

import (
	"gorm.io/gorm"
)

type CourseOwner struct {
	gorm.Model
	ID       int
	UserId   int
	User     User `gorm:"constraint:OnUpdate:CASCADE,OnDelete:SET NULL;"`
	CourseID int
	Course   Course `gorm:"constraint:OnUpdate:CASCADE,OnDelete:SET NULL;"`
}
