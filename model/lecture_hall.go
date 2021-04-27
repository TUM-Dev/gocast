package model

import "gorm.io/gorm"

type LectureHall struct {
	gorm.Model
	Name    string
	CombIP  string
	PresIP  string
	CamIP   string
	Streams []Stream
}
