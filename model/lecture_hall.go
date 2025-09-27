package model

import (
	"fmt"
	"net/netip"
	"net/url"

	"gorm.io/gorm"
)

type LectureHall struct {
	gorm.Model

	Name           string         `gorm:"not null"`            // as in smp (e.g. room_00_13_009A)
	FullName       string         `gorm:"not null"`            // e.g. '5613.EG.009A (00.13.009A, Seminarraum), Boltzmannstr. 3(5613), 85748 Garching b. München'
	StreamProtocol StreamProtocol `gorm:"not null; default:1"` // 1 = rtsp, 2 = srt
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

type StreamProtocol uint

const (
	RTSP StreamProtocol = iota + 1
	SRT
)

type CameraType uint

const (
	Axis CameraType = iota + 1
	Panasonic
)

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

// BeforeSave returns an error if either source is invalid.
func (l *LectureHall) BeforeSave(*gorm.DB) error {
	_, err := netip.ParseAddr(l.CameraIP)
	if err != nil {
		return fmt.Errorf("invalid camera IP address: %s", l.CameraIP)
	}
	u, err := url.Parse(l.CombIP)
	if err != nil {
		return fmt.Errorf("invalid comb URL: %w", err)
	}
	l.CombIP = u.String() // save parsed (and urlencoded) URL to database

	u, err = url.Parse(l.CamIP)
	if err != nil {
		return fmt.Errorf("invalid cam URL: %w", err)
	}
	l.CamIP = u.String()

	u, err = url.Parse(l.PresIP)
	if err != nil {
		return fmt.Errorf("invalid pres URL: %w", err)
	}
	l.PresIP = u.String()
	return nil
}
