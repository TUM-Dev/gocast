package model

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/jinzhu/now"
	"github.com/microcosm-cc/bluemonday"
	"github.com/russross/blackfriday/v2"
	"gorm.io/gorm"
)

// StreamStatus is the status of a stream (e.g. converting)
type StreamStatus int

const (
	StatusUnknown    StreamStatus = iota + 1 // StatusUnknown is the default status of a stream
	StatusConverting                         // StatusConverting indicates that a worker is currently converting the stream.
	StatusConverted                          // StatusConverted indicates that the stream has been converted.
)

type Stream struct {
	gorm.Model

	Name             string
	Description      string
	CourseID         uint
	Start            time.Time `gorm:"not null"`
	End              time.Time `gorm:"not null"`
	RoomName         string
	RoomCode         string
	EventTypeName    string
	TUMOnlineEventID uint
	SeriesIdentifier string `gorm:"default:null"`
	StreamKey        string `gorm:"not null"`
	PlaylistUrl      string
	PlaylistUrlPRES  string
	PlaylistUrlCAM   string
	FilePath         string //deprecated
	LiveNow          bool   `gorm:"not null"`
	Recording        bool
	Premiere         bool `gorm:"default:null"`
	Ended            bool `gorm:"default:null"`
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
	Duration         uint32           `gorm:"default:null"`
	StreamWorkers    []Worker         `gorm:"many2many:stream_workers;"`
	StreamProgresses []StreamProgress `gorm:"foreignKey:StreamID"`
	VideoSections    []VideoSection
	StreamStatus     StreamStatus     `gorm:"not null;default:1"`

	Watched bool `gorm:"-"` // Used to determine if stream is watched when loaded for a specific user.
}

func (s Stream) IsConverting() bool {
	return s.StreamStatus == StatusConverting
}

// IsDownloadable returns true if the stream is a recording and has at least one file associated with it.
func (s Stream) IsDownloadable() bool {
	return s.Recording && len(s.Files) > 0
}

// IsSelfStream returns whether the stream is a scheduled stream in a lecture hall
func (s Stream) IsSelfStream() bool {
	return s.LectureHallID == 0
}

// IsPast returns whether the stream end time was reached
func (s Stream) IsPast() bool {
	return s.End.Before(time.Now()) || s.Ended
}

// IsComingUp returns whether the stream begins in 30 minutes
func (s Stream) IsComingUp() bool {
	eligibleForWait := s.Start.Before(time.Now().Add(30*time.Minute)) && time.Now().Before(s.End)
	return !s.IsPast() && !s.Recording && !s.LiveNow && eligibleForWait
}

// TimeSlotReached returns whether stream has passed the starting time
func (s Stream) TimeSlotReached() bool {
	// Used to stop displaying the timer when there is less than 1 minute left
	return time.Now().After(s.Start.Add(-time.Minute)) && time.Now().Before(s.End)
}

// IsStartingInOneDay returns whether the stream starts within 1 day
func (s Stream) IsStartingInOneDay() bool {
	return s.Start.After(time.Now().Add(24 * time.Hour))
}

// IsStartingInMoreThanOneDay returns whether the stream starts in at least 2 days
func (s Stream) IsStartingInMoreThanOneDay() bool {
	return s.Start.After(time.Now().Add(48 * time.Hour))
}

// IsPlanned returns whether the stream is planned or not
func (s Stream) IsPlanned() bool {
	return !s.Recording && !s.LiveNow && !s.IsPast() && !s.IsComingUp()
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

func (s Stream) FriendlyDate() string {
	return s.Start.Format("Mon 02.01.2006")
}

func (s Stream) FriendlyTime() string {
	return s.Start.Format("02.01.2006 15:04") + " - " + s.End.Format("15:04")
}

func (s Stream) FriendlyNextDate() string {
	if now.With(s.Start).EndOfDay() == now.EndOfDay() {
		return fmt.Sprintf("Today, %02d:%02d", s.Start.Hour(), s.Start.Minute())
	}
	if now.With(s.Start).EndOfDay() == now.With(time.Now().Add(time.Hour*24)).EndOfDay() {
		return fmt.Sprintf("Tomorrow, %02d:%02d", s.Start.Hour(), s.Start.Minute())
	}
	return s.Start.Format("Mon, January 02. 15:04")
}
