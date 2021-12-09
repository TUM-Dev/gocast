package model

import (
	"encoding/json"
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
	FilePath         string //deprecated
	LiveNow          bool
	Recording        bool
	Premiere         bool `gorm:"default:null"`
	Chats            []Chat
	Stats            []Stat
	Units            []StreamUnit
	VodViews         uint `gorm:"default:0"` // todo: remove me before next semester
	StartOffset      uint `gorm:"default:null"`
	EndOffset        uint `gorm:"default:null"`
	LectureHallID    uint `gorm:"default:null"`
	Silences         []Silence
	Files            []File `gorm:"foreignKey:StreamID"`
	Paused           bool   `gorm:"default:false"`
	StreamName       string
}

// IsRegularStream returns whether the stream is a scheduled stream in a lecture hall
func (s Stream) IsRegularStream() bool {
	return s.LectureHallID != 0
}

// IsPast returns whether the stream end time was reached
func (s Stream) IsPast() bool {
	return s.End.Before(time.Now())
}

// IsComingUp returns whether the stream begins in 30 minutes
func (s Stream) IsComingUp() bool {
	eligibleForWait := s.Start.Before(time.Now().Add(30*time.Minute)) && time.Now().Before(s.End)
	return !s.IsPast() && !s.Recording && !s.LiveNow && eligibleForWait
}

func (s Stream) StartsInOneDay() bool {
	return s.Start.After(time.Now().Add(24 * time.Hour))
}

func (s Stream) StartsInMoreThanOneDay() bool {
	return s.Start.After(time.Now().Add(48 * time.Hour))
}

type silence struct {
	Start uint `json:"start"`
	End   uint `json:"end"`
}

func (s Stream) GetSilencesJson() string {
	forServe := make([]silence, len(s.Silences))
	for i := range forServe {
		forServe[i] = silence{
			Start: s.Silences[i].Start,
			End:   s.Silences[i].End,
		}
	}
	if m, err := json.Marshal(forServe); err == nil {
		return string(m)
	}
	return "[]"
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
	return s.Start.Format("20060102T150405")
}

func (s Stream) FriendlyDate() string {
	return s.Start.Format("Mon 02.01.2006")
}

func (s Stream) IsoEnd() string {
	return s.End.Format("20060102T150405")
}

func (s Stream) FriendlyTime() string {
	return s.Start.Format("02.01.2006 15:04") + " - " + s.End.Format("15:04")
}

func (s Stream) IsoCreated() string {
	return s.Model.CreatedAt.Format("20060102T150405")
}
