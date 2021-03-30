package model

import (
	"gorm.io/gorm"
	"time"
)

type Stream struct {
	gorm.Model

	Name             string
	Description      string
	CourseID         uint
	Start            time.Time
	End              time.Time
	RoomName         string
	RoomCode         string
	EventTypeName    string
	TUMOnlineEventID uint
	StreamKey        string
	PlaylistUrl      string
	LiveNow          bool
	Recording        bool
	Chats            []Chat
	Stats            []Stat
}

func (s Stream) IsPast() bool {
	return s.Start.Before(time.Now())
}
