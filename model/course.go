package model

import (
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
	"log"
	"time"
)

// SourceMode 0 -> COMB, 1-> PRES, 2 -> CAM
type SourceMode int

type Course struct {
	gorm.Model

	UserID                  uint   `gorm:"not null"` // Owner of the course
	Name                    string `gorm:"not null"`
	Slug                    string `gorm:"not null"` // eg. eidi
	Year                    int    `gorm:"not null"` // eg. 2021
	TeachingTerm            string `gorm:"not null"` // eg. Summer/Winter
	TUMOnlineIdentifier     string
	VODEnabled              bool `gorm:"default:true"`
	DownloadsEnabled        bool `gorm:"default:false"`
	ChatEnabled             bool `gorm:"default:false"`
	AnonymousChatEnabled    bool `gorm:"not null;default:true"`
	ModeratedChatEnabled    bool `gorm:"not null;default:false"`
	VodChatEnabled          bool
	Visibility              string `gorm:"default:loggedin"` // public, loggedin or enrolled
	Streams                 []Stream
	Users                   []User `gorm:"many2many:course_users;"`
	Admins                  []User `gorm:"many2many:course_admins;"`
	Token                   string
	UserCreatedByToken      bool   `gorm:"default:false"`
	CameraPresetPreferences string // json encoded. e.g. [{lectureHallID:1, presetID:4}, ...]
	SourcePreferences       string // json encoded. e.g. [{lectureHallID:1, sourceMode:0}, ...]
	Pinned                  bool   `gorm:"-"` // Used to determine if the course is pinned when loaded for a specific user.

	LivePrivate bool `gorm:"not null; default:false"` // whether Livestreams are private
	VodPrivate  bool `gorm:"not null; default:false"` // Whether VODs are made private after livestreams
}

type CourseDTO struct {
	ID           uint
	Name         string
	Slug         string
	TeachingTerm string
	Year         int
	NextLecture  StreamDTO
	LastLecture  StreamDTO
	Streams      []StreamDTO
}

func (c *Course) ToDTO() CourseDTO {
	return CourseDTO{
		ID:           c.ID,
		Name:         c.Name,
		Slug:         c.Slug,
		TeachingTerm: c.TeachingTerm,
		Year:         c.Year,
		NextLecture:  c.GetNextLecture().ToDTO(),
		LastLecture:  c.GetLastLecture().ToDTO(),
	}
}

// GetUrl returns the URL of the course, e.g. /course/2022/S/MyCourse
func (c Course) GetUrl() string {
	return fmt.Sprintf("/course/%d/%s/%s", c.Year, c.TeachingTerm, c.Slug)
}

// GetStreamUrl returns the URL of the stream, e.g. /w/MyStream/42
func (c Course) GetStreamUrl(stream Stream) string {
	return fmt.Sprintf("/w/%s/%d", c.Slug, stream.ID)
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

type SourcePreference struct {
	LectureHallID uint       `json:"lecture_hall_id"`
	SourceMode    SourceMode `json:"source_mode"`
}

// GetSourcePreference retrieves the source preferences
func (c Course) GetSourcePreference() []SourcePreference {
	var res []SourcePreference
	err := json.Unmarshal([]byte(c.SourcePreferences), &res)
	if err != nil {
		return []SourcePreference{}
	}
	return res
}

// GetSourceModeForLectureHall retrieves the source preference for the given lecture hall, returns default SourcePreference if non-existing
func (c Course) GetSourceModeForLectureHall(id uint) SourceMode {
	for _, preference := range c.GetSourcePreference() {
		if preference.LectureHallID == id {
			return preference.SourceMode
		}
	}
	return 0
}

// CanUseSource returns whether the specified source type is allowed for the lecture hall id given
func (c Course) CanUseSource(lectureHallID uint, sourceType string) bool {
	mode := c.GetSourceModeForLectureHall(lectureHallID)
	switch sourceType {
	case "PRES":
		return mode != 2
	case "CAM":
		return mode != 1
	case "COMB":
		return mode != 1 && mode != 2
	}
	return true
}

// SetSourcePreference updates the source preferences
func (c *Course) SetSourcePreference(pref []SourcePreference) {
	pBytes, err := json.Marshal(pref)
	if err != nil {
		logrus.WithError(err).Error("Could not marshal source preference")
		return
	}
	c.SourcePreferences = string(pBytes)
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

func (c Course) StreamTimes() []string {
	streamTimes := make([]string, len(c.Streams))

	for i, s := range c.Streams {
		streamTimes[i] = s.Start.In(time.UTC).Format("2006-01-02T15:04:05") + ".000Z"
	}

	return streamTimes
}

// HasRecordings returns whether the course has any recordings.
func (c Course) HasRecordings() bool {
	for i := range c.Streams {
		if c.Streams[i].Recording {
			return true
		}
	}
	return false
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

// GetLastLecture returns the most recent lecture of the course
// Assumes an ascending order of c.Streams
func (c Course) GetLastLecture() Stream {
	var lastLecture Stream
	now := time.Now()
	for _, s := range c.Streams {
		if s.Start.After(now) {
			return lastLecture
		}
		lastLecture = s
	}
	return lastLecture
}

// GetLiveStreams returns the current live streams of the course or an empty slice if none are live
func (c Course) GetLiveStreams() []Stream {
	var res []Stream
	for _, s := range c.Streams {
		if s.LiveNow {
			res = append(res, s)
		}
	}
	return res
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

// HasStreams checks whether the lecture has any streams (recorded, live or upcoming) associated to it
func (c Course) HasStreams() bool {
	return len(c.Streams) > 0
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

// AdminJson is the JSON representation of a courses streams for the admin panel
func (c Course) AdminJson(lhs []LectureHall) []gin.H {
	var res []gin.H
	for _, s := range c.Streams {
		res = append(res, s.getJson(lhs, c))
	}
	return res
}
