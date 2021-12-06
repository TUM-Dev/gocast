package model

import (
	"encoding/json"
	"gorm.io/gorm"
	"log"
	"time"
)

type Course struct {
	gorm.Model

	UserID                  uint   `gorm:"not null"`
	Name                    string `gorm:"not null"`
	Slug                    string `gorm:"not null"` // eg. eidi
	Year                    int    `gorm:"not null"` // eg. 2021
	TeachingTerm            string // eg. Summer/Winter
	TUMOnlineIdentifier     string
	LiveEnabled             bool `gorm:"default:true"`
	VODEnabled              bool
	DownloadsEnabled        bool
	ChatEnabled             bool
	AnonymousChatEnabled    bool `gorm:"not null;default:true"`
	VodChatEnabled          bool
	Visibility              string // public, loggedin or enrolled
	Streams                 []Stream
	Users                   []User `gorm:"many2many:course_users;"`
	Token                   string
	UserCreatedByToken      bool   `gorm:"default:false"`
	CameraPresetPreferences string // json encoded. e.g. [{lectureHallID:1, presetID:4}, ...]
}

type CameraPresetPreference struct {
	LectureHallID uint `json:"lecture_hall_id"`
	PresetID      int  `json:"preset_id"`
}

func (c Course) GetCameraPresetPreference() []CameraPresetPreference {
	var res []CameraPresetPreference
	err := json.Unmarshal([]byte(c.CameraPresetPreferences), &res)
	if err != nil {
		return []CameraPresetPreference{}
	}
	return res
}

func (c *Course) SetCameraPresetPreference(pref []CameraPresetPreference) {
	pBytes, err := json.Marshal(pref)
	if err != nil {
		log.Println(err)
	}
	c.CameraPresetPreferences = string(pBytes)
}

// CompareTo used for sorting. Falling back to old java habits...
func (c Course) CompareTo(other Course) bool {
	if !other.HasNextLecture() {
		return true
	}
	return c.GetNextLectureDate().Before(other.GetNextLectureDate())
}

func (c Course) IsLive() bool {
	for _, s := range c.Streams {
		if s.LiveNow {
			return true
		}
	}
	return false
}

//NumStreams returns the number of streams for the course that are VoDs or live
func (c Course) NumStreams() int {
	res := 0
	for i := range c.Streams {
		if c.Streams[i].Recording || c.Streams[i].LiveNow {
			res++
		}
	}
	return res
}

//NumUsers returns the number of users enrolled in the course
func (c Course) NumUsers() int {
	return len(c.Users)
}

func (c Course) GetNextLectureDate() time.Time {
	earliestLecture := time.Now().Add(time.Hour * 24 * 365 * 10) // 10 years from now.
	for _, s := range c.Streams {
		if s.Start.Before(earliestLecture) && s.End.After(time.Now()) {
			earliestLecture = s.Start
		}
	}
	return earliestLecture
}

func (c Course) GetNextLecture() *Stream {
	earliestLectureDate := time.Now().Add(time.Hour * 24 * 365 * 10) // 10 years from now.
	var earliestLecture *Stream
	for _, s := range c.Streams {
		if s.Start.Before(earliestLectureDate) && s.End.After(time.Now()) {
			earliestLectureDate = s.Start
			earliestLecture = &s
		}
	}
	return earliestLecture
}

func (c Course) HasNextLecture() bool {
	n := time.Now()
	for _, s := range c.Streams {
		if s.Start.After(n) {
			return true
		}
	}
	return false
}

func (c Course) GetRecordings() []Stream {
	var recordings []Stream
	for _, s := range c.Streams {
		if s.Recording {
			recordings = append(recordings, s)
		}
	}
	return recordings
}
