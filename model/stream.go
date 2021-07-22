package model

import (
	"encoding/json"
	"github.com/microcosm-cc/bluemonday"
	"github.com/russross/blackfriday/v2"
	"gorm.io/gorm"
	"strings"
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
	VodViews         uint `gorm:"default:0"` // todo: remove me before next semester
	StartOffset      uint `gorm:"default:null"`
	EndOffset        uint `gorm:"default:null"`
	LectureHallID    uint `gorm:"default:null"`
	Silences         []Silence
}

func (s Stream) GetUrlSourceCOMB() string {
	return strings.ReplaceAll(s.PlaylistUrl, "{{quality}}", "")
}

func (s Stream) GetUrl720COMB() string {
	return strings.ReplaceAll(s.PlaylistUrl, "{{quality}}", "_720p")
}

func (s Stream) GetUrl360COMB() string {
	return strings.ReplaceAll(s.PlaylistUrl, "{{quality}}", "_360p")
}

func (s Stream) GetUrlSourcePRES() string {
	return strings.ReplaceAll(s.PlaylistUrlPRES, "{{quality}}", "")
}

func (s Stream) GetUrl720PRES() string {
	return strings.ReplaceAll(s.PlaylistUrlPRES, "{{quality}}", "_720p")
}

func (s Stream) GetUrl360PRES() string {
	return strings.ReplaceAll(s.PlaylistUrlPRES, "{{quality}}", "_360p")
}

func (s Stream) GetUrlSourceCAM() string {
	return strings.ReplaceAll(s.PlaylistUrlCAM, "{{quality}}", "")
}

func (s Stream) GetUrl720CAM() string {
	return strings.ReplaceAll(s.PlaylistUrlCAM, "{{quality}}", "_720p")
}

func (s Stream) GetUrl360CAM() string {
	return strings.ReplaceAll(s.PlaylistUrlCAM, "{{quality}}", "_360p")
}

func (s Stream) IsPast() bool {
	return s.End.Before(time.Now())
}

type silence struct {
	Start uint `json:"start"`
	End   uint `json:"end"`
}

func (s Stream) GetSilencesJson() string {
	forServe := make([]silence, len(s.Silences))
	for i, _ := range forServe {
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

func (s Stream) IsoEnd() string {
	return s.End.Format("20060102T150405")
}

func (s Stream) IsoCreated() string {
	return s.Model.CreatedAt.Format("20060102T150405")
}
