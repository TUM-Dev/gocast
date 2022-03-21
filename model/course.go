package model

import (
	"encoding/json"
	"gorm.io/gorm"
	"log"
	"time"
)

type Course struct {
	gorm.Model

	UserID                  uint   `gorm:"not null"` // Owner of the course
	Name                    string `gorm:"not null"`
	Slug                    string `gorm:"not null"` // eg. eidi
	Year                    int    `gorm:"not null"` // eg. 2021
	TeachingTerm            string `gorm:"not null"` // eg. Summer/Winter
	TUMOnlineIdentifier     string
	LiveEnabled             bool `gorm:"default:true"`
	VODEnabled              bool `gorm:"default:true"`
	DownloadsEnabled        bool `gorm:"default:false"`
	ChatEnabled             bool `gorm:"default:false"`
	AnonymousChatEnabled    bool `gorm:"not null;default:true"`
	ModeratedChatEnabled    bool `gorm:"not null;default:false"`
	VodChatEnabled          bool
	Visibility              string `gorm:"default:loggedin"` // public, loggedin or enrolled
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

// GetCameraPresetPreference retrieves the camera preset preferences
func (c Course) GetCameraPresetPreference() []CameraPresetPreference {
	var res []CameraPresetPreference
	err := json.Unmarshal([]byte(c.CameraPresetPreferences), &res)
	if err != nil {
		return []CameraPresetPreference{}
	}
	return res
}

// SetCameraPresetPreference updates the camera preset preferences
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

// IsLive returns whether the course has a lecture that is live
func (c Course) IsLive() bool {
	for _, s := range c.Streams {
		if s.LiveNow {
			return true
		}
	}
	return false
}

// IsNextLectureStartingSoon checks whether the course has a lecture that starts soon
func (c Course) IsNextLectureStartingSoon() bool {
	for _, s := range c.Streams {
		if s.IsComingUp() {
			return true
		}
	}
	return false
}

// NumStreams returns the number of streams for the course that are VoDs or live
func (c Course) NumStreams() int {
	res := 0
	for i := range c.Streams {
		if c.Streams[i].Recording || c.Streams[i].LiveNow {
			res++
		}
	}
	return res
}

// NumUsers returns the number of users enrolled in the course
func (c Course) NumUsers() int {
	return len(c.Users)
}

// NextLectureHasReachedTimeSlot returns whether the courses next lecture arrived at its timeslot
func (c Course) NextLectureHasReachedTimeSlot() bool {
	return c.GetNextLecture().TimeSlotReached()
}

// GetNextLecture returns the next lecture of the course
func (c Course) GetNextLecture() Stream {
	var earliestLecture Stream
	earliestLectureDate := time.Now().Add(time.Hour * 24 * 365 * 10) // 10 years from now.
	for _, s := range c.Streams {
		if s.Start.Before(earliestLectureDate) && s.End.After(time.Now()) {
			earliestLectureDate = s.Start
			earliestLecture = s
		}
	}
	return earliestLecture
}

// GetLiveStream returns the current live stream of the course (if any)
func (c Course) GetLiveStream() *Stream {
	for _, s := range c.Streams {
		if s.LiveNow {
			return &s
		}
	}
	return nil
}

// GetNextLectureDate returns the next lecture date of the course
func (c Course) GetNextLectureDate() time.Time {
	// TODO: Refactor this with IsNextLectureSelfStream when the sorting error fixed
	earliestLectureDate := time.Now().Add(time.Hour * 24 * 365 * 10) // 10 years from now.
	for _, s := range c.Streams {
		if s.Start.Before(earliestLectureDate) && s.End.After(time.Now()) {
			earliestLectureDate = s.Start
		}
	}
	return earliestLectureDate
}

// IsNextLectureSelfStream checks whether the next lecture is a self stream
func (c Course) IsNextLectureSelfStream() bool {
	return c.GetNextLecture().IsSelfStream()
}

// GetNextLectureDateFormatted returns a JavaScript friendly formatted date string
func (c Course) GetNextLectureDateFormatted() string {
	return c.GetNextLectureDate().Format("2006-01-02 15:04:05")
}

// HasNextLecture checks whether there is another upcoming lecture
func (c Course) HasNextLecture() bool {
	n := time.Now()
	for _, s := range c.Streams {
		if s.Start.After(n) {
			return true
		}
	}
	return false
}

// GetRecordings returns all recording of this course as streams
func (c Course) GetRecordings() []Stream {
	var recordings []Stream
	for _, s := range c.Streams {
		if s.Recording {
			recordings = append(recordings, s)
		}
	}
	return recordings
}

// IsHidden returns true if visibility is set to 'hidden' and false if not
func (c Course) IsHidden() bool {
	return c.Visibility == "hidden"
}
