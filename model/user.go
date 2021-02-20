package model

import "gorm.io/gorm"

// Users struct is a row record of the users table in the rbglive database
type User struct {
	gorm.Model
	ID int
	Name string
	Email string
	Role string
	Password string
}
