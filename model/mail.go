package model

import "gorm.io/gorm"

type Mail struct {
	gorm.Model
	To   string
	Body string
}
