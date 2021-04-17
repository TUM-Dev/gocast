package model

import "gorm.io/gorm"

type StreamUnit struct {
	gorm.Model
	UnitName        string
	UnitDescription string
	UnitStart       uint
	UnitEnd         uint
	StreamID        uint
}
