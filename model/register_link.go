package model

import "gorm.io/gorm"

type RegisterLink struct {
	gorm.Model
	ID             int
	UserId         int
	User           User
	RegisterSecret string
}
