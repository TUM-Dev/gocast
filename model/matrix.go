package model

import "gorm.io/gorm"

type MatrixData struct {
	gorm.Model

	token string
}
