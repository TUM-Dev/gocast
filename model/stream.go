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
	PlaylistUrlPRES  string
	PlaylistUrlCAM   string
	FilePath         string
	LiveNow          bool
	Recording        bool
	Chats            []Chat
	Stats            []Stat
}

func (s Stream) IsPast() bool {
	return s.End.Before(time.Now())
}
