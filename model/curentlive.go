package model

import "gorm.io/gorm"

type CurrentLive struct {
	gorm.Model
	ID int32
	Url string
}
