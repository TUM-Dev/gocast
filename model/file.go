package model

import "gorm.io/gorm"

type File struct {
	gorm.Model

	StreamID uint
	Path     string
}
