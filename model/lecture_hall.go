package model

import "gorm.io/gorm"

type LectureHall struct {
	gorm.Model

	Name           string `gorm:"not null"` // as in smp (e.g. room_00_13_009A)
	FullName       string `gorm:"not null"` // e.g. '5613.EG.009A (00.13.009A, Seminarraum), Boltzmannstr. 3(5613), 85748 Garching b. München'
	CombIP         string
	PresIP         string
	CamIP          string
	CameraIP       string     // ip of the actual camera (not smp)
	CameraType     CameraType `gorm:"not null; default:1"`
	Streams        []Stream
	CameraPresets  []CameraPreset
	RoomID         int    // used by TUMOnline
	PwrCtrlIp      string // power control api for red live light
	LiveLightIndex int    // id of power outlet for live light
	ExternalURL    string
}

type CameraType uint

const (
	Axis CameraType = iota + 1
	Panasonic
)

func (l *LectureHall) NumSources() int {
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

type LectureHallDTO struct {
	ID          uint
	Name        string
	ExternalURL string
}

func (l *LectureHall) ToDTO() *LectureHallDTO {
	if l == nil {
		return nil
	}
	return &LectureHallDTO{
		ID:          l.ID,
		Name:        l.Name,
		ExternalURL: l.ExternalURL,
	}
}
