package model

import "gorm.io/gorm"

type LectureHall struct {
	gorm.Model
	Name          string
	CombIP        string
	PresIP        string
	CamIP         string
	CameraIP      string // ip of the actual camera (not smp)
	Streams       []Stream
	CameraPresets []CameraPreset
}
