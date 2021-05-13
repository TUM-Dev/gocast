package model

import (
	"fmt"
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
	VodViews         uint `gorm:"default:0"`
	StartOffset      uint `gorm:"default:null"`
	EndOffset        uint `gorm:"default:null"`
	LectureHallID    uint `gorm:"default:null"`
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

func (s Stream) IsoStart() string {
	return fmt.Sprintf("%04d%02d%02dT%02d%02d%02d", s.Start.Year(), s.Start.Month(), s.Start.Day(), s.Start.Hour(), s.Start.Minute(), s.Start.Second())
}

func (s Stream) IsoEnd() string {
	return fmt.Sprintf("%04d%02d%02dT%02d%02d%02d", s.End.Year(), s.End.Month(), s.End.Day(), s.End.Hour(), s.End.Minute(), s.End.Second())
}
