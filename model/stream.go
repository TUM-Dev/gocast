package model

import (
	"github.com/microcosm-cc/bluemonday"
	"github.com/russross/blackfriday/v2"
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
	Units            []StreamUnit
	StartOffset      uint `gorm:"default:null"`
	EndOffset        uint `gorm:"default:null"`
}

func (s Stream) IsPast() bool {
	return s.End.Before(time.Now())
}

func (s Stream) GetDescriptionHTML() string {
	unsafe := blackfriday.Run([]byte(s.Description))
	html := bluemonday.
		UGCPolicy().
		AddTargetBlankToFullyQualifiedLinks(true).
		SanitizeBytes(unsafe)
	return string(html)
}
