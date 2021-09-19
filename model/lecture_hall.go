package model

import "gorm.io/gorm"

type LectureHall struct {
	gorm.Model
	Name           string
	CombIP         string
	PresIP         string
	CamIP          string
	CameraIP       string // ip of the actual camera (not smp)
	Streams        []Stream
	CameraPresets  []CameraPreset
	PwrCtrlIp      string // power control api for red live light
	LiveLightIndex int    // id of power outlet for live light
}

func (l LectureHall) NumSources() int {
	num := 0
	if l.CombIP != "" {
		num++
	}
	if l.PresIP != "" {
		num++
	}
	if l.CamIP != "" {
		num++
	}
	return num
}
